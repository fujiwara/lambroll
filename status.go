package lambroll

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/samber/lo"
)

// StatusOption represents options for Status()
type StatusOption struct {
	Qualifier *string `help:"compare with"`
	Output    string  `default:"text" enum:"text,json" help:"output format"`
}

type FunctionStatusOutput struct {
	Configuration *types.FunctionConfiguration
	Code          *types.FunctionCodeLocation
	Tags          Tags
	FunctionURL   *types.FunctionUrlConfig
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
		Qualifier:    opt.Qualifier,
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

	if res, err := app.lambda.GetFunctionUrlConfig(ctx, &lambda.GetFunctionUrlConfigInput{
		FunctionName: fn.FunctionName,
		Qualifier:    opt.Qualifier,
	}); err != nil {
		var nfe *types.ResourceNotFoundException
		if errors.As(err, &nfe) {
			// ignore
			log.Println("[debug] FunctionUrlConfig not found")
		} else {
			return fmt.Errorf("failed to get function url config: %w", err)
		}
	} else {
		log.Println("[debug] FunctionUrlConfig found")
		st.FunctionURL = &types.FunctionUrlConfig{
			FunctionUrl: res.FunctionUrl,
			AuthType:    res.AuthType,
			Cors:        res.Cors,
			InvokeMode:  res.InvokeMode,
		}
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
	tags := make([]string, 0, len(st.Tags))
	for k, v := range st.Tags {
		tags = append(tags, fmt.Sprintf("%s=%s", k, v))
	}
	archs := make([]string, 0, len(st.Configuration.Architectures))
	for _, a := range st.Configuration.Architectures {
		archs = append(archs, string(a))
	}
	loggingConfig := []string{
		"  LogFormat: " + string(st.Configuration.LoggingConfig.LogFormat),
		"  LogGroup: " + aws.ToString(st.Configuration.LoggingConfig.LogGroup),
	}
	if lv := string(st.Configuration.LoggingConfig.ApplicationLogLevel); lv != "" {
		loggingConfig = append(loggingConfig, "  ApplicationLogLevel: "+lv)
	}
	if lv := string(st.Configuration.LoggingConfig.SystemLogLevel); lv != "" {
		loggingConfig = append(loggingConfig, "  SystemLogLevel: "+lv)
	}
	var snapStart string
	if ss := st.Configuration.SnapStart; ss != nil {
		snapStart = strings.Join([]string{
			"  ApplyOn: " + string(ss.ApplyOn),
			"  OptimizationStatus: " + string(ss.OptimizationStatus),
		}, "\n")
	}

	res := []string{
		"FunctionName: " + aws.ToString(st.Configuration.FunctionName),
		"Description: " + aws.ToString(st.Configuration.Description),
		"Version: " + aws.ToString(st.Configuration.Version),
		"FunctionArn: " + aws.ToString(st.Configuration.FunctionArn),
		"Role: " + aws.ToString(st.Configuration.Role),
		"State: " + string(st.Configuration.State),
		"LastUpdateStatus: " + string(st.Configuration.LastUpdateStatus),
		"LoggingConfig: \n" + strings.Join(loggingConfig, "\n"),
		"SnapStart: \n" + snapStart,
		"Architectures: " + strings.Join(archs, ","),
		"Runtime: " + string(st.Configuration.Runtime),
		"Handler: " + aws.ToString(st.Configuration.Handler),
		"Timeout: " + fmt.Sprintf("%d", aws.ToInt32(st.Configuration.Timeout)),
		"MemorySize: " + fmt.Sprintf("%d", aws.ToInt32(st.Configuration.MemorySize)),
		"PackageType: " + string(st.Configuration.PackageType),
		"CodeSize: " + fmt.Sprintf("%d", st.Configuration.CodeSize),
		"CodeSha256: " + aws.ToString(st.Configuration.CodeSha256),
		"Tags: " + strings.Join(tags, ","),
	}

	if st.FunctionURL != nil {
		res = append(res, []string{
			"FunctionUrl:",
			"  FunctionUrl: " + aws.ToString(st.FunctionURL.FunctionUrl),
			"  AuthType: " + string(st.FunctionURL.AuthType),
			"  InvokeMode: " + string(st.FunctionURL.InvokeMode),
		}...)
		if cors := st.FunctionURL.Cors; cors != nil {
			res = append(res, "  Cors:", formatCors(cors, 4))
		}
	}
	return strings.Join(res, "\n") + "\n"
}

func formatCors(cors *types.Cors, indentLevel int) string {
	if cors == nil {
		return ""
	}
	indent := strings.Repeat(" ", indentLevel)
	res := lo.Map([]string{
		"AllowCredentials: " + fmt.Sprintf("%t", aws.ToBool(cors.AllowCredentials)),
		"AllowOrigins: " + strings.Join(cors.AllowOrigins, ","),
		"AllowHeaders: " + strings.Join(cors.AllowHeaders, ","),
		"AllowMethods: " + strings.Join(cors.AllowMethods, ","),
		"ExposeHeaders: " + strings.Join(cors.ExposeHeaders, ","),
		"MaxAge: " + fmt.Sprintf("%d", aws.ToInt32(cors.MaxAge)),
	}, func(item string, _ int) string {
		return indent + item
	})
	return strings.Join(res, "\n")
}
