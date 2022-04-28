package lambroll

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/fatih/color"
	"github.com/kylelemons/godebug/diff"
	"github.com/pkg/errors"
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
	excludes, err := expandExcludeFile(*opt.ExcludeFile)
	if err != nil {
		return errors.Wrap(err, "failed to parse exclude-file")
	}
	opt.Excludes = append(opt.Excludes, excludes...)

	newFunc, err := app.loadFunction(*opt.FunctionFilePath)
	if err != nil {
		return errors.Wrap(err, "failed to load function")
	}
	fillDefaultValues(newFunc)
	name := *newFunc.FunctionName

	var latest *lambda.FunctionConfiguration
	var code *lambda.FunctionCodeLocation

	var tags Tags
	var currentCodeSha256, packageType string
	if res, err := app.lambda.GetFunction(&lambda.GetFunctionInput{
		FunctionName: &name,
	}); err != nil {
		return errors.Wrapf(err, "failed to GetFunction %s", name)
	} else {
		latest = res.Configuration
		code = res.Code
		tags = res.Tags
		currentCodeSha256 = *res.Configuration.CodeSha256
		packageType = *res.Configuration.PackageType
	}
	latestFunc := newFunctionFrom(latest, code, tags)

	latestJSON, _ := marshalJSON(latestFunc)
	newJSON, _ := marshalJSON(newFunc)

	if ds := diff.Diff(string(latestJSON), string(newJSON)); ds != "" {
		fmt.Println(color.RedString("---" + app.functionArn(name)))
		fmt.Println(color.GreenString("+++" + *opt.FunctionFilePath))
		fmt.Println(coloredDiff(ds))
	}

	if err := validateUpdateFunction(latest, code, newFunc); err != nil {
		return err
	}

	if aws.BoolValue(opt.CodeSha256) {
		if strings.ToLower(packageType) != "zip" {
			return errors.New("code-sha256 is only supported for Zip package type")
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
