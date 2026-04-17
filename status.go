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
	"github.com/olekukonko/tablewriter/tw"
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

func (o *StatusOutput) Table() (string, error) {
	buf := new(strings.Builder)
	w := tablewriter.NewTable(buf, tablewriter.WithRendition(tw.Rendition{
		Symbols: tw.NewSymbols(tw.StyleASCII),
	}))
	rows := [][2]string{
		{"FunctionName", o.FunctionName},
		{"FunctionArn", o.FunctionArn},
		{"Version", o.Version},
	}
	if o.Runtime != "" {
		rows = append(rows, [2]string{"Runtime", o.Runtime})
	}
	rows = append(rows,
		[2]string{"PackageType", o.PackageType},
		[2]string{"State", o.State},
		[2]string{"LastUpdateState", o.LastUpdateState},
	)
	if o.FunctionURL != "" {
		rows = append(rows, [2]string{"FunctionURL", o.FunctionURL})
	}
	for _, r := range rows {
		if err := w.Append(r[0], r[1]); err != nil {
			return "", fmt.Errorf("failed to append row: %w", err)
		}
	}
	if err := w.Render(); err != nil {
		return "", fmt.Errorf("failed to render table: %w", err)
	}
	return buf.String(), nil
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
		tbl, err := out.Table()
		if err != nil {
			return err
		}
		fmt.Print(tbl)
	case "json":
		b, _ := marshalJSON(out)
		fmt.Print(string(b))
	}
	return nil
}
