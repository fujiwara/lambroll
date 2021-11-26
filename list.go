package lambroll

import (
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/pkg/errors"
)

// ListOption represents options for List()
type ListOption struct {
}

// List lists lambda functions
func (app *App) List(opt ListOption) error {
	var marker *string
	for {
		res, err := app.lambda.ListFunctions(&lambda.ListFunctionsInput{
			MaxItems: aws.Int64(50),
		})
		if err != nil {
			return errors.Wrap(err, "failed to ListFunctions")
		}
		for _, c := range res.Functions {
			arn := app.functionArn(*c.FunctionName)
			log.Printf("[debug] listing tags of %s", arn)
			res, err := app.lambda.ListTags(&lambda.ListTagsInput{
				Resource: aws.String(arn),
			})
			if err != nil {
				return errors.Wrap(err, "faled to list tags")
			}
			b, _ := marshalJSON(newFunctionFrom(c, res.Tags))
			os.Stdout.Write(b)
		}
		if marker = res.NextMarker; marker == nil {
			break
		}
	}
	return nil
}
