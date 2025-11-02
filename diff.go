package lambroll

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/aereal/jsondiff"
	"github.com/fatih/color"
	"github.com/itchyny/gojq"
	"github.com/kylelemons/godebug/diff"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

// DiffOption represents options for Diff()
type DiffOption struct {
	Src          string  `help:"function zip archive or src dir" default:"."`
	CodeSha256   bool    `name:"code" help:"diff of code sha256" default:"false"`
	Qualifier    *string `help:"the qualifier to compare"`
	FunctionURL  string  `help:"path to function-url definition" default:"" env:"LAMBROLL_FUNCTION_URL"`
	Ignore       string  `help:"ignore diff by jq query" default:""`
	ExitCode     bool    `help:"exit with code 2 if there are differences" default:"false"`
	SkipFunction bool    `help:"skip function diff. shows function-url only" default:"false"`

	ZipOption
}

// Diff prints diff of function.json compared with latest function
func (app *App) Diff(ctx context.Context, opt *DiffOption) error {
	if err := opt.Expand(); err != nil {
		return err
	}
	fn, err := app.loadFunction(app.functionFilePath)
	if err != nil {
		return fmt.Errorf("failed to load function: %w", err)
	}
	fillDefaultValues(fn)
	name := aws.ToString(fn.FunctionName)

	// function diff
	hasDiff, err := app.diffFunction(ctx, fn, opt)
	if err != nil {
		return err
	}

	// function-url diff
	if d, err := app.diffFunctionURL(ctx, name, opt); err != nil {
		return err
	} else if d {
		hasDiff = true
	}

	if hasDiff && opt.ExitCode {
		// exit with code 2 if there are differences
		// but actually, it's not an error
		return ErrDiff
	}
	return nil
}

func (app *App) diffFunction(ctx context.Context, fn *Function, opt *DiffOption) (bool, error) {
	if opt.SkipFunction {
		return false, nil
	}
	name := aws.ToString(fn.FunctionName)
	var remote *types.FunctionConfiguration
	var code *types.FunctionCodeLocation

	var tags Tags
	var currentCodeSha256 string
	var packageType types.PackageType
	if res, err := app.lambda.GetFunction(ctx, &lambda.GetFunctionInput{
		FunctionName: &name,
		Qualifier:    opt.Qualifier,
	}); err != nil {
		var nfe *types.ResourceNotFoundException
		if errors.As(err, &nfe) {
			slog.Info("function not found. lambroll deploy will create a new function", "function", name)
		} else {
			return false, fmt.Errorf("failed to GetFunction %s: %w", name, err)
		}
	} else {
		remote = res.Configuration
		code = res.Code
		{
			slog.Debug("list tags", "resource", app.functionArn(ctx, name))
			res, err := app.lambda.ListTags(ctx, &lambda.ListTagsInput{
				// Tagging operations are permitted on Lambda functions only.
				// Tags on aliases and versions are not supported.
				Resource: aws.String(app.functionArn(ctx, name)),
			})
			if err != nil {
				return false, fmt.Errorf("failed to list tags: %w", err)
			}
			tags = res.Tags
		}
		currentCodeSha256 = *res.Configuration.CodeSha256
		packageType = res.Configuration.PackageType
	}
	remoteFunc := newFunctionFrom(remote, code, tags)
	fillDefaultValues(remoteFunc)

	opts := []jsondiff.Option{}
	if ignore := opt.Ignore; ignore != "" {
		if p, err := gojq.Parse(ignore); err != nil {
			return false, fmt.Errorf("failed to parse ignore query: %s %w", ignore, err)
		} else {
			opts = append(opts, jsondiff.Ignore(p))
		}
	}

	remoteJSON, _ := marshalAny(remoteFunc)
	newJSON, _ := marshalAny(fn)
	remoteArn := fullQualifiedFunctionName(app.functionArn(ctx, name), opt.Qualifier)
	hasDiff := false

	if diff, err := jsondiff.Diff(
		&jsondiff.Input{Name: remoteArn, X: remoteJSON},
		&jsondiff.Input{Name: app.functionFilePath, X: newJSON},
		opts...,
	); err != nil {
		return false, fmt.Errorf("failed to diff: %w", err)
	} else if diff != "" {
		hasDiff = true
		fmt.Print(coloredDiff(diff))
	}

	if err := validateUpdateFunction(remote, code, fn); err != nil {
		return false, err
	}

	if opt.CodeSha256 {
		if packageType != types.PackageTypeZip {
			return false, fmt.Errorf("code-sha256 is only supported for Zip package type")
		}
		zipfile, _, err := prepareZipfile(opt.Src, opt.excludes, opt.KeepSymlink)
		if err != nil {
			return false, err
		}
		h := sha256.New()
		if _, err := io.Copy(h, zipfile); err != nil {
			return false, err
		}
		newCodeSha256 := base64.StdEncoding.EncodeToString(h.Sum(nil))
		prefix := "CodeSha256: "
		if ds := diff.Diff(prefix+currentCodeSha256, prefix+newCodeSha256); ds != "" {
			fmt.Println(color.RedString("---" + app.functionArn(ctx, name)))
			fmt.Println(color.GreenString("+++" + "--src=" + opt.Src))
			fmt.Println(coloredDiff(ds))
			hasDiff = true
		}
	}
	return hasDiff, nil
}

func (app *App) diffFunctionURL(ctx context.Context, name string, opt *DiffOption) (bool, error) {
	if opt.FunctionURL == "" {
		// skip function-url diff
		return false, nil
	}
	var remote, local *types.FunctionUrlConfig
	var hasDiff bool

	fu, err := app.loadFunctionUrl(opt.FunctionURL, name)
	if err != nil {
		return hasDiff, fmt.Errorf("failed to load function-url: %w", err)
	} else {
		fillDefaultValuesFunctionUrlConfig(fu.Config)
		local = &types.FunctionUrlConfig{
			AuthType:   fu.Config.AuthType,
			Cors:       fu.Config.Cors,
			InvokeMode: fu.Config.InvokeMode,
		}
	}
	var qualifier *string
	if opt.Qualifier != nil {
		qualifier = opt.Qualifier
	} else if fu.Config != nil && fu.Config.Qualifier != nil {
		qualifier = fu.Config.Qualifier
	}
	fqName := fullQualifiedFunctionName(name, qualifier)

	if res, err := app.lambda.GetFunctionUrlConfig(ctx, &lambda.GetFunctionUrlConfigInput{
		FunctionName: &name,
		Qualifier:    qualifier,
	}); err != nil {
		var nfe *types.ResourceNotFoundException
		if errors.As(err, &nfe) {
			// empty
			remote = &types.FunctionUrlConfig{}
		} else {
			return hasDiff, fmt.Errorf("failed to get function url config: %w", err)
		}
	} else {
		slog.Debug("FunctionUrlConfig found")
		remote = &types.FunctionUrlConfig{
			AuthType:   res.AuthType,
			Cors:       res.Cors,
			InvokeMode: res.InvokeMode,
		}
	}
	r, _ := toGeneralMap(remote, true)
	l, _ := toGeneralMap(local, true)

	if diff, err := jsondiff.Diff(
		&jsondiff.Input{Name: fqName, X: r},
		&jsondiff.Input{Name: opt.FunctionURL, X: l},
	); err != nil {
		return hasDiff, fmt.Errorf("failed to diff: %w", err)
	} else if diff != "" {
		hasDiff = true
		fmt.Print(coloredDiff(diff))
	}

	// permissions
	adds, removes, err := app.calcFunctionURLPermissionsDiff(ctx, fu)
	if err != nil {
		return hasDiff, err
	}
	var addsB []byte
	for _, in := range adds {
		b, _ := marshalJSON(in)
		addsB = append(addsB, b...)
	}
	var removesB []byte
	for _, in := range removes {
		b, _ := marshalJSON(in)
		removesB = append(removesB, b...)
	}
	if ds := diff.Diff(string(removesB), string(addsB)); ds != "" {
		fmt.Println(color.RedString("--- permissions"))
		fmt.Println(color.GreenString("+++ permissions"))
		fmt.Print(coloredDiff(ds))
		hasDiff = true
	}

	return hasDiff, nil
}

func coloredDiff(src string) string {
	var b strings.Builder
	for _, line := range strings.Split(src, "\n") {
		if strings.HasPrefix(line, "-") {
			b.WriteString(color.RedString(line) + "\n")
		} else if strings.HasPrefix(line, "+") {
			b.WriteString(color.GreenString(line) + "\n")
		} else {
			b.WriteString(line + "\n")
		}
	}
	return b.String()
}
