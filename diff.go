package lambroll

import (
	"fmt"

	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/fatih/color"
	"github.com/kylelemons/godebug/diff"
	"github.com/pkg/errors"
)

// DiffOption represents options for Diff()
type DiffOption struct {
	FunctionFilePath *string
}

// Diff prints diff of function.json compared with latest function
func (app *App) Diff(opt DiffOption) error {
	newFunc, err := app.loadFunction(*opt.FunctionFilePath)
	if err != nil {
		return errors.Wrap(err, "failed to load function")
	}
	fillDefaultValues(newFunc)
	name := *newFunc.FunctionName

	var latest *lambda.FunctionConfiguration
	var code *lambda.FunctionCodeLocation

	var tags Tags
	if res, err := app.lambda.GetFunction(&lambda.GetFunctionInput{
		FunctionName: &name,
	}); err != nil {
		return errors.Wrapf(err, "failed to GetFunction %s", name)
	} else {
		latest = res.Configuration
		code = res.Code
		tags = res.Tags
	}
	latestFunc := newFunctionFrom(latest, code, tags)

	latestJSON, _ := marshalJSON(latestFunc)
	newJSON, _ := marshalJSON(newFunc)

	if ds := diff.Diff(string(latestJSON), string(newJSON)); ds != "" {
		fmt.Println(color.RedString("---", app.functionArn(name)))
		fmt.Println(color.GreenString("+++", *opt.FunctionFilePath))
		fmt.Println(ds)
	}
	return nil
}
