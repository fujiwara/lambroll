package lambroll

import (
	"context"
	"fmt"
	"log"

	"github.com/Songmu/prompter"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
)

// DeleteOption represents options for Delete()
type DeleteOption struct {
	DryRun bool `help:"dry run" default:"false" negatable:""`
	Force  bool `help:"delete without confirmation" default:"false"`
}

func (opt DeleteOption) label() string {
	if opt.DryRun {
		return "**DRY RUN**"
	}
	return ""
}

// Delete deletes function
func (app *App) Delete(ctx context.Context, opt *DeleteOption) error {
	fn, err := app.loadFunction(app.functionFilePath)
	if err != nil {
		return fmt.Errorf("failed to load function: %w", err)
	}

	log.Println("[info] deleting function", *fn.FunctionName, opt.label())

	if opt.DryRun {
		return nil
	}

	if !opt.Force && !prompter.YN("Do you want to delete the function?", false) {
		log.Println("[info] canceled to delete function", *fn.FunctionName)
		return nil
	}

	_, err = app.lambda.DeleteFunction(ctx, &lambda.DeleteFunctionInput{
		FunctionName: fn.FunctionName,
	})
	if err != nil {
		return fmt.Errorf("failed to delete function: %w", err)
	}

	log.Println("[info] completed to delete function", *fn.FunctionName)

	return nil
}
