package lambroll

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
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
	Src        string `help:"function zip archive or src dir" default:"."`
	CodeSha256 bool   `help:"diff of code sha256" default:"false"`
	Qualifier  string `help:"compare with" default:"$LATEST"`
	Ignore     string `help:"ignore diff by jq query" default:""`

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
		Qualifier:    &opt.Qualifier,
	}); err != nil {
		return fmt.Errorf("failed to GetFunction %s: %w", name, err)
	} else {
		remote = res.Configuration
		code = res.Code
		{
			res, err := app.lambda.ListTags(ctx, &lambda.ListTagsInput{
				// Tagging operations are permitted on Lambda functions only.
				// Tags on aliases and versions are not supported.
				Resource: aws.String(app.functionArn(ctx, name)),
			})
			if err != nil {
				return fmt.Errorf("faled to list tags: %w", err)
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
	remoteArn := app.functionArn(ctx, name) + ":" + opt.Qualifier

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

	if opt.CodeSha256 {
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
