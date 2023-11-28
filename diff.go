package lambroll

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/fatih/color"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"github.com/kylelemons/godebug/diff"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

// DiffOption represents options for Diff()
type DiffOption struct {
	Src        string `help:"function zip archive or src dir" default:"."`
	CodeSha256 bool   `help:"diff of code sha256" default:"false"`
	Unified    bool   `help:"unified diff" default:"true" negatable:"" short:"u"`
	Qualifier  string `help:"compare with" default:"$LATEST"`

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

	var latest *types.FunctionConfiguration
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
		latest = res.Configuration
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
	latestFunc := newFunctionFrom(latest, code, tags)

	latestJSON, _ := marshalJSON(latestFunc)
	newJSON, _ := marshalJSON(newFunc)
	remoteArn := app.functionArn(ctx, name) + ":" + opt.Qualifier

	if opt.Unified {
		edits := myers.ComputeEdits(span.URIFromPath(remoteArn), string(latestJSON), string(newJSON))
		if ds := fmt.Sprint(gotextdiff.ToUnified(remoteArn, app.functionFilePath, string(latestJSON), edits)); ds != "" {
			fmt.Print(coloredDiff(ds))
		}
	} else {
		if ds := diff.Diff(string(latestJSON), string(newJSON)); ds != "" {
			fmt.Println(color.RedString("---" + remoteArn))
			fmt.Println(color.GreenString("+++" + app.functionFilePath))
			fmt.Print(coloredDiff(ds))
		}
	}

	if err := validateUpdateFunction(latest, code, newFunc); err != nil {
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
