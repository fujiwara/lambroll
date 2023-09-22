package lambroll

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/service/lambda"
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
func (app *App) Delete(ctx context.Context, opt DeleteOption) error {
	fn, err := app.loadFunction(*opt.FunctionFilePath)
	if err != nil {
		return fmt.Errorf("failed to load function: %w", err)
	}

	log.Println("[info] deleting function", *fn.FunctionName, opt.label())

	if *opt.DryRun {
		return nil
	}
	_, err = app.lambda.DeleteFunction(ctx, &lambda.DeleteFunctionInput{
		FunctionName: fn.FunctionName,
	})
	if err != nil {
		return fmt.Errorf("failed to delete function: %w", err)
	}

	return nil
}
