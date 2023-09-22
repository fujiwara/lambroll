package lambroll

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
)

// ListOption represents options for List()
type ListOption struct {
}

// List lists lambda functions
func (app *App) List(ctx context.Context, opt ListOption) error {
	var marker *string
	for {
		res, err := app.lambda.ListFunctions(ctx, &lambda.ListFunctionsInput{
			MaxItems: aws.Int32(50),
		})
		if err != nil {
			return fmt.Errorf("failed to ListFunctions: %w", err)
		}
		for _, c := range res.Functions {
			arn := app.functionArn(ctx, *c.FunctionName)
			log.Printf("[debug] listing tags of %s", arn)
			res, err := app.lambda.ListTags(ctx, &lambda.ListTagsInput{
				Resource: aws.String(arn),
			})
			if err != nil {
				return fmt.Errorf("faled to list tags: %w", err)
			}
			b, _ := marshalJSON(newFunctionFrom(&c, nil, res.Tags))
			os.Stdout.Write(b)
		}
		if marker = res.NextMarker; marker == nil {
			break
		}
	}
	return nil
}
