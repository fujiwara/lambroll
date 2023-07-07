package lambroll

import (
	"context"
	"fmt"
	"log"

	lambdav2 "github.com/aws/aws-sdk-go-v2/service/lambda"
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
	ctx := context.TODO()
	fn, err := app.loadFunction(*opt.FunctionFilePath)
	if err != nil {
		return fmt.Errorf("failed to load function: %w", err)
	}

	log.Println("[info] deleting function", *fn.FunctionName, opt.label())

	if *opt.DryRun {
		return nil
	}
	_, err = app.lambdav2.DeleteFunction(ctx, &lambdav2.DeleteFunctionInput{
		FunctionName: fn.FunctionName,
	})
	if err != nil {
		return fmt.Errorf("failed to delete function: %w", err)
	}

	return nil
}
