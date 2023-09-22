package lambroll

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"github.com/fatih/color"
	"github.com/kylelemons/godebug/diff"

	awsv2 "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

// DiffOption represents options for Diff()
type DiffOption struct {
	FunctionFilePath *string
	Src              *string
	Excludes         []string
	CodeSha256       *bool
	ExcludeFile      *string
}

// Diff prints diff of function.json compared with latest function
func (app *App) Diff(opt DiffOption) error {
	ctx := context.TODO()
	excludes, err := expandExcludeFile(*opt.ExcludeFile)
	if err != nil {
		return fmt.Errorf("failed to parse exclude-file: %w", err)
	}
	opt.Excludes = append(opt.Excludes, excludes...)

	newFunc, err := app.loadFunctionV2(*opt.FunctionFilePath)
	if err != nil {
		return fmt.Errorf("failed to load function: %w", err)
	}
	fillDefaultValuesV2(newFunc)
	name := *newFunc.FunctionName

	var latest *types.FunctionConfiguration
	var code *types.FunctionCodeLocation

	var tags TagsV2
	var currentCodeSha256 string
	var packageType types.PackageType
	if res, err := app.lambda.GetFunction(ctx, &lambda.GetFunctionInput{
		FunctionName: &name,
	}); err != nil {
		return fmt.Errorf("failed to GetFunction %s: %w", name, err)
	} else {
		latest = res.Configuration
		code = res.Code
		tags = res.Tags
		currentCodeSha256 = *res.Configuration.CodeSha256
		packageType = res.Configuration.PackageType
	}
	latestFunc := newFunctionFromV2(latest, code, tags)

	latestJSON, _ := marshalJSONV2(latestFunc)
	newJSON, _ := marshalJSONV2(newFunc)

	if ds := diff.Diff(string(latestJSON), string(newJSON)); ds != "" {
		fmt.Println(color.RedString("---" + app.functionArn(name)))
		fmt.Println(color.GreenString("+++" + *opt.FunctionFilePath))
		fmt.Println(coloredDiff(ds))
	}

	if err := validateUpdateFunctionV2(latest, code, newFunc); err != nil {
		return err
	}

	if awsv2.ToBool(opt.CodeSha256) {
		if packageType != types.PackageTypeZip {
			return fmt.Errorf("code-sha256 is only supported for Zip package type")
		}
		zipfile, _, err := prepareZipfile(*opt.Src, opt.Excludes)
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
			fmt.Println(color.RedString("---" + app.functionArn(name)))
			fmt.Println(color.GreenString("+++" + "--src=" + *opt.Src))
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
