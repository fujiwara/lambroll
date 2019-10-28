package lambroll

import (
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
		for _, fn := range res.Functions {
			b, _ := marshalJSON(fn)
			os.Stdout.Write(b)
		}
		if marker = res.NextMarker; marker == nil {
			break
		}
	}
	return nil
}
