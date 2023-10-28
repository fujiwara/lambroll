package lambroll

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

type FunctionURLConfig = lambda.CreateFunctionUrlConfigInput

func (app *App) loadFunctionUrlConfig(path string) (*FunctionURLConfig, error) {
	return loadDefinitionFile[FunctionURLConfig](app, path, DefaultFunctionURLConfigFilenames)
}

func (app *App) deployFunctionURL(ctx context.Context, functionName, path string) error {
	fc, err := app.loadFunctionUrlConfig(path)
	if err != nil {
		return fmt.Errorf("failed to load function url config: %w", err)
	}
	fc.FunctionName = &functionName
	log.Println("[info] deploying function url config...")

	_, err = app.lambda.GetFunctionUrlConfig(ctx, &lambda.GetFunctionUrlConfigInput{
		FunctionName: &functionName,
		Qualifier:    fc.Qualifier,
	})
	create := false
	if err != nil {
		var nfe *types.ResourceNotFoundException
		if errors.As(err, &nfe) {
			log.Printf("[info] function url config for %s not found. creating", functionName)
			create = true
		} else {
			return fmt.Errorf("failed to get function url config: %w", err)
		}
	}
	if create {
		res, err := app.lambda.CreateFunctionUrlConfig(ctx, fc)
		if err != nil {
			return fmt.Errorf("failed to create function url config: %w", err)
		}
		log.Printf("[info] created function url config for %s", functionName)
		log.Printf("[info] Function URL: %s", *res.FunctionUrl)
	} else {
		res, err := app.lambda.UpdateFunctionUrlConfig(ctx, &lambda.UpdateFunctionUrlConfigInput{
			FunctionName: fc.FunctionName,
			Qualifier:    fc.Qualifier,
			AuthType:     fc.AuthType,
			Cors:         fc.Cors,
		})
		if err != nil {
			return fmt.Errorf("failed to update function url config: %w", err)
		}
		log.Printf("[info] updated function url config for %s", functionName)
		log.Printf("[info] Function URL: %s", *res.FunctionUrl)
	}

	{
		res, err := app.lambda.GetPolicy(ctx, &lambda.GetPolicyInput{
			FunctionName: fc.FunctionName,
			Qualifier:    fc.Qualifier,
		})
		if err != nil {
			return fmt.Errorf("failed to get policy: %w", err)
		}
		log.Printf("[info] policy for %s: %s", functionName, *res.Policy)
	}

	// TODO add or update permission
	log.Println("[info] adding permission...")
	if _, err := app.lambda.AddPermission(ctx, &lambda.AddPermissionInput{
		Action:              aws.String("lambda:InvokeFunctionUrl"),
		FunctionName:        fc.FunctionName,
		Qualifier:           fc.Qualifier,
		FunctionUrlAuthType: fc.AuthType,
		StatementId:         aws.String("lambroll"),
		Principal:           aws.String("*"), // TODO
	}); err != nil {
		return fmt.Errorf("failed to add permission: %w", err)
	}
	log.Printf("[info] added permission for %s", functionName)

	return nil
}
