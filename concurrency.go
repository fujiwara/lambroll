package lambroll

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

// unreservedConcurrency is the value of ReservedConcurrentExecutions in function.json
// that means removing the reserved concurrency of the function.
const unreservedConcurrency int32 = -1

type concurrencyAction int

const (
	concurrencyActionNone concurrencyAction = iota
	concurrencyActionPut
	concurrencyActionDelete
)

// planReservedConcurrency decides how to update the reserved concurrency of the function.
// local is ReservedConcurrentExecutions in function.json, current is the value of the deployed function (nil if not set).
func planReservedConcurrency(local, current *int32) (concurrencyAction, error) {
	if local == nil {
		return concurrencyActionNone, nil
	}
	switch v := *local; {
	case v == unreservedConcurrency:
		if current == nil {
			return concurrencyActionNone, nil
		}
		return concurrencyActionDelete, nil
	case v < 0:
		return concurrencyActionNone, fmt.Errorf("invalid ReservedConcurrentExecutions %d: must be >= 0, or %d to remove", v, unreservedConcurrency)
	default:
		if current != nil && *current == v {
			return concurrencyActionNone, nil
		}
		return concurrencyActionPut, nil
	}
}

func validateReservedConcurrency(fn *Function) error {
	_, err := planReservedConcurrency(fn.ReservedConcurrentExecutions, nil)
	return err
}

func (app *App) updateReservedConcurrency(ctx context.Context, fn *Function, opt *DeployOption) error {
	logger := opt.logger()

	if fn.ReservedConcurrentExecutions == nil {
		logger.Debug("ReservedConcurrentExecutions not defined in function.json skip updating reserved concurrency")
		return nil
	}
	current, err := app.getReservedConcurrency(ctx, *fn.FunctionName)
	if err != nil {
		return err
	}

	action, err := planReservedConcurrency(fn.ReservedConcurrentExecutions, current)
	if err != nil {
		return err
	}
	switch action {
	case concurrencyActionPut:
		logger.Info("setting reserved concurrency", "value", *fn.ReservedConcurrentExecutions)
		if opt.DryRun {
			return nil
		}
		return app.withRetryOnConflict(ctx, "put function concurrency", func(ctx context.Context) error {
			_, err := app.lambda.PutFunctionConcurrency(ctx, &lambda.PutFunctionConcurrencyInput{
				FunctionName:                 fn.FunctionName,
				ReservedConcurrentExecutions: fn.ReservedConcurrentExecutions,
			})
			return err
		})
	case concurrencyActionDelete:
		logger.Info("removing reserved concurrency", "current", aws.ToInt32(current))
		if opt.DryRun {
			return nil
		}
		return app.withRetryOnConflict(ctx, "delete function concurrency", func(ctx context.Context) error {
			_, err := app.lambda.DeleteFunctionConcurrency(ctx, &lambda.DeleteFunctionConcurrencyInput{
				FunctionName: fn.FunctionName,
			})
			return err
		})
	default:
		logger.Debug("no need to update reserved concurrency (unchanged)")
		return nil
	}
}

// getReservedConcurrency returns the reserved concurrency of the function.
// It returns nil if the reserved concurrency is not set or the function does not exist.
// GetFunction with a qualifier does not return Concurrency, so use GetFunctionConcurrency.
func (app *App) getReservedConcurrency(ctx context.Context, name string) (*int32, error) {
	res, err := app.lambda.GetFunctionConcurrency(ctx, &lambda.GetFunctionConcurrencyInput{
		FunctionName: aws.String(name),
	})
	if err != nil {
		var nfe *types.ResourceNotFoundException
		if errors.As(err, &nfe) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get function concurrency: %w", err)
	}
	return res.ReservedConcurrentExecutions, nil
}

func (app *App) withRetryOnConflict(ctx context.Context, msg string, proc func(ctx context.Context) error) error {
	retryer := retryPolicy.Start(ctx)
	for retryer.Continue() {
		err := proc(ctx)
		if err == nil {
			return nil
		}
		var rce *types.ResourceConflictException
		if errors.As(err, &rce) {
			slog.Debug("retrying", "error", rce.Error())
			continue
		}
		return fmt.Errorf("failed to %s: %w", msg, err)
	}
	return fmt.Errorf("failed to %s (max retries reached)", msg)
}
