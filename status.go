package lambroll

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
)

// StatusOption represents options for Status()
type StatusOption struct {
	Qualifier *string `help:"compare with"`
}

// Status prints status of function
func (app *App) Status(ctx context.Context, opt *StatusOption) error {
	fn, err := app.loadFunction(app.functionFilePath)
	if err != nil {
		return fmt.Errorf("failed to load function: %w", err)
	}
	name := *fn.FunctionName

	res, err := app.lambda.GetFunction(ctx, &lambda.GetFunctionInput{
		FunctionName: &name,
		Qualifier:    opt.Qualifier,
	})
	if err != nil {
		return fmt.Errorf("failed to GetFunction %s: %w", name, err)
	}
	fmt.Printf("FunctionName: %s\n", aws.ToString(res.Configuration.FunctionName))
	fmt.Printf("Version: %s\n", aws.ToString(res.Configuration.Version))
	fmt.Printf("FunctionArn: %s\n", aws.ToString(res.Configuration.FunctionArn))
	fmt.Printf("State: %s\n", string(res.Configuration.State))
	fmt.Printf("LastUpdateStatus: %s\n", string(res.Configuration.LastUpdateStatus))
	return nil
}
