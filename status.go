package lambroll

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/olekukonko/tablewriter"
)

// StatusOption represents options for Status()
type StatusOption struct {
	Qualifier *string `help:"compare with"`
	Output    string  `help:"output format" default:"table" enum:"table,json"`
}

type StatusOutput struct {
	FunctionName    string `json:"FunctionName"`
	FunctionArn     string `json:"FunctionArn"`
	Version         string `json:"Version"`
	Runtime         string `json:"Runtime,omitempty"`
	PackageType     string `json:"PackageType"`
	State           string `json:"State"`
	LastUpdateState string `json:"LastUpdateState"`
	FunctionURL     string `json:"FunctionURL,omitempty"`
}

func (o *StatusOutput) String() string {
	buf := new(strings.Builder)
	w := tablewriter.NewWriter(buf)
	w.Append([]string{"FunctionName", o.FunctionName})
	w.Append([]string{"FunctionArn", o.FunctionArn})
	w.Append([]string{"Version", o.Version})
	if o.Runtime != "" {
		w.Append([]string{"Runtime", o.Runtime})
	}
	w.Append([]string{"PackageType", o.PackageType})
	w.Append([]string{"State", o.State})
	w.Append([]string{"LastUpdateState", o.LastUpdateState})
	if o.FunctionURL != "" {
		w.Append([]string{"FunctionURL", o.FunctionURL})
	}
	w.Render()
	return buf.String()
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
	out := &StatusOutput{
		FunctionName:    aws.ToString(res.Configuration.FunctionName),
		FunctionArn:     aws.ToString(res.Configuration.FunctionArn),
		Version:         aws.ToString(res.Configuration.Version),
		Runtime:         string(res.Configuration.Runtime),
		PackageType:     string(res.Configuration.PackageType),
		State:           string(res.Configuration.State),
		LastUpdateState: string(res.Configuration.LastUpdateStatus),
	}
	if res, err := app.lambda.GetFunctionUrlConfig(ctx, &lambda.GetFunctionUrlConfigInput{
		FunctionName: &name,
		Qualifier:    opt.Qualifier,
	}); err != nil {
		var nfe *types.ResourceNotFoundException
		if errors.As(err, &nfe) {
			// do nothing
		} else {
			return fmt.Errorf("failed to GetFunctionUrlConfig %s: %w", name, err)
		}
	} else {
		out.FunctionURL = aws.ToString(res.FunctionUrl)
	}
	switch opt.Output {
	case "table":
		fmt.Print(out.String())
	case "json":
		b, _ := marshalJSON(out)
		fmt.Print(string(b))
	}
	return nil
}
