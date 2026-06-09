package lambroll

import (
	"context"
	"fmt"

	"github.com/Songmu/prompter"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
)

// DeleteOption represents options for Delete()
type DeleteOption struct {
	Force bool `help:"delete without confirmation" default:"false" json:"force,omitempty"`

	DryRunOption
}

// Delete deletes function
func (app *App) Delete(ctx context.Context, opt *DeleteOption) error {
	logger := opt.logger()

	fn, err := app.loadFunction(app.functionFilePath)
	if err != nil {
		return fmt.Errorf("failed to load function: %w", err)
	}

	logger.Info("deleting function", "function", *fn.FunctionName)

	if opt.DryRun {
		return nil
	}

	if !opt.Force && !prompter.YN("Do you want to delete the function?", false) {
		logger.Info("canceled to delete function", "function", *fn.FunctionName)
		return nil
	}

	_, err = app.lambda.DeleteFunction(ctx, &lambda.DeleteFunctionInput{
		FunctionName: fn.FunctionName,
	})
	if err != nil {
		return fmt.Errorf("failed to delete function: %w", err)
	}

	logger.Info("completed to delete function", "function", *fn.FunctionName)

	return nil
}
