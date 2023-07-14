package lambroll

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	lambdav2 "github.com/aws/aws-sdk-go-v2/service/lambda"
)

// ListOption represents options for List()
type ListOption struct {
}

// List lists lambda functions
func (app *App) List(opt ListOption) error {
	ctx := context.TODO()
	var marker *string
	for {
		res, err := app.lambdav2.ListFunctions(ctx, &lambdav2.ListFunctionsInput{
			MaxItems: aws.Int32(50),
		})
		if err != nil {
			return fmt.Errorf("failed to ListFunctions: %w", err)
		}
		for _, c := range res.Functions {
			arn := app.functionArn(*c.FunctionName)
			log.Printf("[debug] listing tags of %s", arn)
			res, err := app.lambdav2.ListTags(ctx, &lambdav2.ListTagsInput{
				Resource: aws.String(arn),
			})
			if err != nil {
				return fmt.Errorf("faled to list tags: %w", err)
			}
			b, _ := marshalJSONV2(newFunctionFromV2(&c, nil, res.Tags))
			os.Stdout.Write(b)
		}
		if marker = res.NextMarker; marker == nil {
			break
		}
	}
	return nil
}
