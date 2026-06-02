package lambroll

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aereal/jsondiff"
	"github.com/fatih/color"
	"github.com/itchyny/gojq"
	"github.com/mattn/go-shellwords"

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
	External     string  `help:"external command to display diff" default:"" env:"LAMBROLL_DIFF_COMMAND"`

	ZipOption

	w io.Writer `kong:"-"`
}

// Diff prints diff of function.json compared with latest function
func (app *App) Diff(ctx context.Context, opt *DiffOption) error {
	if err := opt.Expand(); err != nil {
		return err
	}
	if opt.w == nil {
		opt.w = os.Stdout
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

	// AWS returns VPC subnet/security group IDs in an arbitrary order, so
	// normalize the ordering on both sides to avoid a spurious diff.
	sortFunctionForDiff(fn)
	sortFunctionForDiff(remoteFunc)

	remoteJSON, _ := marshalAny(remoteFunc)
	newJSON, _ := marshalAny(fn)
	remoteArn := fullQualifiedFunctionName(app.functionArn(ctx, name), opt.Qualifier)
	hasDiff := false

	if d, err := app.emitDiff(ctx, opt, "function.json",
		&jsondiff.Input{Name: remoteArn, X: remoteJSON},
		&jsondiff.Input{Name: app.functionFilePath, X: newJSON},
		opt.Ignore,
	); err != nil {
		return false, err
	} else if d {
		hasDiff = true
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
		if d, err := app.emitDiff(ctx, opt, "CodeSha256.txt",
			&jsondiff.Input{Name: remoteArn, X: prefix + currentCodeSha256},
			&jsondiff.Input{Name: "--src=" + opt.Src, X: prefix + newCodeSha256},
			"",
		); err != nil {
			return false, err
		} else if d {
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

	if d, err := app.emitDiff(ctx, opt, "function_url.json",
		&jsondiff.Input{Name: fqName, X: r},
		&jsondiff.Input{Name: opt.FunctionURL, X: l},
		"",
	); err != nil {
		return hasDiff, err
	} else if d {
		hasDiff = true
	}

	// permissions
	adds, removes, err := app.calcFunctionURLPermissionsDiff(ctx, fu)
	if err != nil {
		return hasDiff, err
	}
	var addsB []*lambda.AddPermissionInput
	for _, in := range adds {
		addsB = append(addsB, in)
	}
	var removesB []*lambda.AddPermissionInput
	for _, in := range removes {
		removesB = append(removesB, in)
	}

	if d, err := app.emitDiff(ctx, opt, "permissions.json",
		&jsondiff.Input{Name: "permissions", X: removesB},
		&jsondiff.Input{Name: "permissions", X: addsB},
		"",
	); err != nil {
		return hasDiff, err
	} else if d {
		hasDiff = true
	}

	return hasDiff, nil
}

// sortFunctionForDiff normalizes order-insensitive slice fields so that
// differences in ordering alone do not produce a spurious diff. AWS treats
// VPC subnet IDs and security group IDs as sets and may return them in an
// arbitrary order.
func sortFunctionForDiff(fn *Function) {
	if fn == nil {
		return
	}
	if v := fn.VpcConfig; v != nil {
		sort.Strings(v.SubnetIds)
		sort.Strings(v.SecurityGroupIds)
	}
}

// emitDiff computes the diff between from and to using jsondiff (honoring the
// ignore jq query) and renders it. When opt.External is set, the diff is shown
// by running the external command against two temporary files instead of the
// built-in colored unified diff. It returns true if there is any difference.
func (app *App) emitDiff(ctx context.Context, opt *DiffOption, label string, from, to *jsondiff.Input, ignore string) (bool, error) {
	var jsonOpts []jsondiff.Option
	if ignore != "" {
		p, err := gojq.Parse(ignore)
		if err != nil {
			return false, fmt.Errorf("failed to parse ignore query: %s %w", ignore, err)
		}
		jsonOpts = append(jsonOpts, jsondiff.Ignore(p))
	}

	diff, err := jsondiff.Diff(from, to, jsonOpts...)
	if err != nil {
		return false, fmt.Errorf("failed to diff: %w", err)
	}
	if diff == "" {
		return false, nil
	}

	if opt.External != "" {
		remote, err := renderForExternalDiff(from.X, ignore)
		if err != nil {
			return false, err
		}
		local, err := renderForExternalDiff(to.X, ignore)
		if err != nil {
			return false, err
		}
		if err := runExternalDiff(ctx, opt.External, label, remote, local, opt.w); err != nil {
			return false, err
		}
		return true, nil
	}

	fmt.Fprint(opt.w, coloredDiff(diff))
	return true, nil
}

// renderForExternalDiff converts a diff input value into the text written to a
// temporary file for the external diff command. When an ignore query is given,
// the same fields ignored by jsondiff are deleted here too, so that the
// external diff does not display ignored differences.
func renderForExternalDiff(x any, ignore string) (string, error) {
	if ignore != "" {
		q, err := gojq.Parse("del(" + ignore + ")")
		if err != nil {
			return "", fmt.Errorf("failed to parse ignore query: %s %w", ignore, err)
		}
		modified, err := jsondiff.ModifyValue(q, x)
		if err != nil {
			return "", fmt.Errorf("failed to apply ignore query: %w", err)
		}
		x = modified
	}
	if s, ok := x.(string); ok {
		return s, nil
	}
	b, err := json.MarshalIndent(x, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal for external diff: %w", err)
	}
	return string(b) + "\n", nil
}

// runExternalDiff writes the remote and local content into temporary files and
// runs the external command with the two file paths appended as arguments.
// The command's stdout is written to w. The command must exit with status 0;
// wrap commands such as diff(1) that exit non-zero on differences.
func runExternalDiff(ctx context.Context, command, label, remote, local string, w io.Writer) error {
	args, err := shellwords.Parse(command)
	if err != nil {
		return fmt.Errorf("failed to parse diff command %q: %w", command, err)
	}
	if len(args) == 0 {
		return fmt.Errorf("empty diff command")
	}

	tmpDir, err := os.MkdirTemp("", "lambroll-diff-")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current dir: %w", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		return fmt.Errorf("failed to chdir: %w", err)
	}
	defer os.Chdir(pwd)

	for _, name := range []string{"remote", "local"} {
		if err := os.Mkdir(name, 0755); err != nil {
			return fmt.Errorf("failed to create %s dir: %w", name, err)
		}
	}
	remoteFile := filepath.Join("remote", label)
	localFile := filepath.Join("local", label)
	if err := os.WriteFile(remoteFile, []byte(remote), 0644); err != nil {
		return fmt.Errorf("failed to write remote file: %w", err)
	}
	if err := os.WriteFile(localFile, []byte(local), 0644); err != nil {
		return fmt.Errorf("failed to write local file: %w", err)
	}
	args = append(args, remoteFile, localFile)

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Stdout = w
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run diff command: %w", err)
	}
	return nil
}

func coloredDiff(src string) string {
	var b strings.Builder
	for line := range strings.SplitSeq(src, "\n") {
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
