package lambroll

import (
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/kylelemons/godebug/diff"
	"github.com/pkg/errors"
)

// DiffOption represents options for Diff()
type DiffOption struct {
	FunctionFilePath *string
}

// Diff prints diff of function.json compared with latest function
func (app *App) Diff(opt DiffOption) error {
	newFunc, err := app.loadFunction(*opt.FunctionFilePath)
	if err != nil {
		return errors.Wrap(err, "failed to load function")
	}
	fillDefaultValues(newFunc)
	name := *newFunc.FunctionName

	var latest *lambda.FunctionConfiguration
	if res, err := app.lambda.GetFunction(&lambda.GetFunctionInput{
		FunctionName: &name,
	}); err != nil {
		return errors.Wrapf(err, "failed to GetFunction %s", name)
	} else {
		latest = res.Configuration
	}

	arn := app.functionArn(name)
	log.Printf("[debug] listing tags of %s", arn)
	var tags Tags
	if res, err := app.lambda.ListTags(&lambda.ListTagsInput{
		Resource: aws.String(arn),
	}); err != nil {
		return errors.Wrap(err, "faled to list tags")
	} else {
		tags = res.Tags
	}
	latestFunc := newFuctionFrom(latest, tags)

	latestJSON, _ := marshalJSON(latestFunc)
	newJSON, _ := marshalJSON(newFunc)

	if ds := diff.Diff(string(latestJSON), string(newJSON)); ds != "" {
		fmt.Println("---", arn)
		fmt.Println("+++", *opt.FunctionFilePath)
		fmt.Println(ds)
	}
	return nil
}
