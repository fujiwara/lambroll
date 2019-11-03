package lambroll

import (
	"log"

	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/pkg/errors"
)

// DeleteOption represents options for Delete()
type DeleteOption struct {
	FunctionFilePath *string
	DryRun           *bool
}

func (opt DeleteOption) label() string {
	if *opt.DryRun {
		return "**DRY RUN**"
	}
	return ""
}

// Delete deletes function
func (app *App) Delete(opt DeleteOption) error {
	fn, err := app.loadFunction(*opt.FunctionFilePath)
	if err != nil {
		return errors.Wrap(err, "failed to load function")
	}

	log.Println("[info] deleting function", *fn.FunctionName, opt.label())

	if *opt.DryRun {
		return nil
	}
	_, err = app.lambda.DeleteFunction(&lambda.DeleteFunctionInput{
		FunctionName: fn.FunctionName,
	})
	if err != nil {
		return errors.Wrap(err, "failed to delete function")
	}

	return nil
}
