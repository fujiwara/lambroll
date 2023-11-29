package lambroll

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

// StatusOption represents options for Status()
type StatusOption struct {
	Qualifier string `help:"compare with" default:"$LATEST"`
	Output    string `default:"text" enum:"text,json" help:"output format"`
}

type FunctionStatusOutput struct {
	Configuration *types.FunctionConfiguration
	Code          *types.FunctionCodeLocation
	Tags          Tags
}

// Status prints status of function
func (app *App) Status(ctx context.Context, opt *StatusOption) error {
	fn, err := app.loadFunction(app.functionFilePath)
	if err != nil {
		return fmt.Errorf("failed to load function: %w", err)
	}
	name := *fn.FunctionName

	var configuration *types.FunctionConfiguration
	var code *types.FunctionCodeLocation
	var tags Tags

	if res, err := app.lambda.GetFunction(ctx, &lambda.GetFunctionInput{
		FunctionName: &name,
		Qualifier:    &opt.Qualifier,
	}); err != nil {
		return fmt.Errorf("failed to GetFunction %s: %w", name, err)
	} else {
		configuration = res.Configuration
		code = res.Code
		{
			log.Println("[debug] list tags Resource", app.functionArn(ctx, name))
			res, err := app.lambda.ListTags(ctx, &lambda.ListTagsInput{
				// Tagging operations are permitted on Lambda functions only.
				// Tags on aliases and versions are not supported.
				Resource: aws.String(app.functionArn(ctx, name)),
			})
			if err != nil {
				return fmt.Errorf("failed to list tags: %w", err)
			}
			tags = res.Tags
		}
	}
	st := &FunctionStatusOutput{
		Configuration: configuration,
		Code:          code,
		Tags:          tags,
	}
	switch opt.Output {
	case "text":
		fmt.Print(st.String())
	case "json":
		b, err := marshalJSON(st)
		if err != nil {
			return fmt.Errorf("failed to marshal json: %w", err)
		}
		fmt.Print(string(b))
	}
	return nil
}

func (st *FunctionStatusOutput) String() string {
	tags := make(map[string]string, len(st.Tags))
	for k, v := range st.Tags {
		tags[k] = v
	}
	res := strings.Join([]string{
		"FunctionName: " + aws.ToString(st.Configuration.FunctionName),
		"Description: " + aws.ToString(st.Configuration.Description),
		"Version: " + aws.ToString(st.Configuration.Version),
		"FunctionArn: " + aws.ToString(st.Configuration.FunctionArn),
		"State: " + string(st.Configuration.State),
		"LastUpdateStatus: " + string(st.Configuration.LastUpdateStatus),
		"PackageType: " + string(st.Configuration.PackageType),
		"Runtime: " + string(st.Configuration.Runtime),
		"Handler: " + aws.ToString(st.Configuration.Handler),
		"Timeout: " + fmt.Sprintf("%d", aws.ToInt32(st.Configuration.Timeout)),
		"MemorySize: " + fmt.Sprintf("%d", aws.ToInt32(st.Configuration.MemorySize)),
		"Role: " + aws.ToString(st.Configuration.Role),
		"LastModified: " + aws.ToString(st.Configuration.LastModified),
		"CodeSize: " + fmt.Sprintf("%d", st.Configuration.CodeSize),
		"CodeSha256: " + aws.ToString(st.Configuration.CodeSha256),
	}, "\n") + "\n"
	if len(tags) > 0 {
		res += "Tags:\n"
		for k, v := range tags {
			res += fmt.Sprintf("  %s: %s\n", k, v)
		}
	}
	return res
}
