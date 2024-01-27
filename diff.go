package lambroll

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
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
	Src         string  `help:"function zip archive or src dir" default:"."`
	CodeSha256  bool    `help:"diff of code sha256" default:"false"`
	Qualifier   *string `help:"the qualifier to compare"`
	FunctionURL string  `help:"path to function-url definiton" default:"" env:"LAMBROLL_FUNCTION_URL"`
	Ignore      string  `help:"ignore diff by jq query" default:""`

	ExcludeFileOption
}

// Diff prints diff of function.json compared with latest function
func (app *App) Diff(ctx context.Context, opt *DiffOption) error {
	if err := opt.Expand(); err != nil {
		return err
	}

	newFunc, err := app.loadFunction(app.functionFilePath)
	if err != nil {
		return fmt.Errorf("failed to load function: %w", err)
	}
	fillDefaultValues(newFunc)
	name := *newFunc.FunctionName

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
			log.Printf("[info] function %s is not found. lambroll deploy will create a new function.", name)
		} else {
			return fmt.Errorf("failed to GetFunction %s: %w", name, err)
		}
	} else {
		remote = res.Configuration
		code = res.Code
		{
			log.Println("[debug] list tags Resource", app.functionArn(ctx, name))
			res, err := app.lambda.ListTags(ctx, &lambda.ListTagsInput{
				// Tagging operations are permitted on Lambda functions only.
				// Tags on aliases and versions are not supported.
				Resource: aws.String(app.functionArn(ctx, name)),
			})
			if err != nil {
				return fmt.Errorf("failed to list tags: %w", err)
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
			return fmt.Errorf("failed to parse ignore query: %s %w", ignore, err)
		} else {
			opts = append(opts, jsondiff.Ignore(p))
		}
	}

	remoteJSON, _ := marshalAny(remoteFunc)
	newJSON, _ := marshalAny(newFunc)
	remoteArn := fullQualifiedFunctionName(app.functionArn(ctx, name), opt.Qualifier)

	if diff, err := jsondiff.Diff(
		&jsondiff.Input{Name: remoteArn, X: remoteJSON},
		&jsondiff.Input{Name: app.functionFilePath, X: newJSON},
		opts...,
	); err != nil {
		return fmt.Errorf("failed to diff: %w", err)
	} else if diff != "" {
		fmt.Print(coloredDiff(diff))
	}

	if err := validateUpdateFunction(remote, code, newFunc); err != nil {
		return err
	}

	if opt.CodeSha256 && latest != nil {
		if packageType != types.PackageTypeZip {
			return fmt.Errorf("code-sha256 is only supported for Zip package type")
		}
		zipfile, _, err := prepareZipfile(opt.Src, opt.excludes)
		if err != nil {
			return err
		}
		h := sha256.New()
		if _, err := io.Copy(h, zipfile); err != nil {
			return err
		}
		newCodeSha256 := base64.StdEncoding.EncodeToString(h.Sum(nil))
		prefix := "CodeSha256: "
		if ds := diff.Diff(prefix+currentCodeSha256, prefix+newCodeSha256); ds != "" {
			fmt.Println(color.RedString("---" + app.functionArn(ctx, name)))
			fmt.Println(color.GreenString("+++" + "--src=" + opt.Src))
			fmt.Println(coloredDiff(ds))
		}
	}

	if opt.FunctionURL == "" {
		return nil
	}

	if err := app.diffFunctionURL(ctx, name, opt); err != nil {
		return err
	}
	return nil
}

func (app *App) diffFunctionURL(ctx context.Context, name string, opt *DiffOption) error {
	var remote, local *types.FunctionUrlConfig
	fqName := fullQualifiedFunctionName(name, opt.Qualifier)

	fu, err := app.loadFunctionUrl(opt.FunctionURL, name)
	if err != nil {
		return fmt.Errorf("failed to load function-url: %w", err)
	} else {
		fillDefaultValuesFunctionUrlConfig(fu.Config)
		local = &types.FunctionUrlConfig{
			AuthType:   fu.Config.AuthType,
			Cors:       fu.Config.Cors,
			InvokeMode: fu.Config.InvokeMode,
		}
	}

	if res, err := app.lambda.GetFunctionUrlConfig(ctx, &lambda.GetFunctionUrlConfigInput{
		FunctionName: &name,
		Qualifier:    opt.Qualifier,
	}); err != nil {
		var nfe *types.ResourceNotFoundException
		if errors.As(err, &nfe) {
			// empty
			remote = &types.FunctionUrlConfig{}
		} else {
			return fmt.Errorf("failed to get function url config: %w", err)
		}
	} else {
		log.Println("[debug] FunctionUrlConfig found")
		remote = &types.FunctionUrlConfig{
			AuthType:   res.AuthType,
			Cors:       res.Cors,
			InvokeMode: res.InvokeMode,
		}
	}
	r, _ := marshalJSON(remote)
	l, _ := marshalJSON(local)

	if opt.Unified {
		edits := myers.ComputeEdits(span.URIFromPath(fqName), string(r), string(l))
		if ds := fmt.Sprint(gotextdiff.ToUnified(fqName, opt.FunctionURL, string(r), edits)); ds != "" {
			fmt.Print(coloredDiff(ds))
		}
	} else {
		if ds := diff.Diff(string(r), string(l)); ds != "" {
			fmt.Println(color.RedString("---" + fqName))
			fmt.Println(color.GreenString("+++" + opt.FunctionURL))
			fmt.Print(coloredDiff(ds))
		}
	}

	// permissions
	adds, removes, err := app.calcFunctionURLPermissionsDiff(ctx, fu)
	if err != nil {
		return err
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
		fmt.Println(color.RedString("---"))
		fmt.Println(color.GreenString("+++"))
		fmt.Print(coloredDiff(ds))
	}

	return nil
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
