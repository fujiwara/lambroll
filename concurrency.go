package lambroll

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

// removeReservedConcurrency is the --reserved-concurrency value to remove the reserved concurrency.
const removeReservedConcurrency int32 = -1

type concurrencyAction int

const (
	concurrencyActionNone concurrencyAction = iota
	concurrencyActionPut
	concurrencyActionDelete
)

func validateReservedConcurrency(v *int32) error {
	if v != nil && *v < removeReservedConcurrency {
		return fmt.Errorf("invalid --reserved-concurrency %d: must be >= 0, or %d to remove", *v, removeReservedConcurrency)
	}
	return nil
}

// planReservedConcurrency decides the action to make the current reserved concurrency the desired one.
func planReservedConcurrency(desired, current *int32) concurrencyAction {
	switch {
	case desired == nil:
		return concurrencyActionNone
	case *desired == removeReservedConcurrency:
		if current == nil {
			return concurrencyActionNone
		}
		return concurrencyActionDelete
	case current != nil && *current == *desired:
		return concurrencyActionNone
	default:
		return concurrencyActionPut
	}
}

func (app *App) deployReservedConcurrency(ctx context.Context, name string, opt *DeployOption) error {
	logger := opt.logger()
	desired := opt.ReservedConcurrency
	if desired == nil {
		return nil
	}

	var current *int32
	res, err := app.lambda.GetFunctionConcurrency(ctx, &lambda.GetFunctionConcurrencyInput{
		FunctionName: aws.String(name),
	})
	if err != nil {
		// the function may not exist yet in dry-run
		if _, ok := errors.AsType[*types.ResourceNotFoundException](err); !ok {
			return fmt.Errorf("failed to get function concurrency: %w", err)
		}
	} else {
		current = res.ReservedConcurrentExecutions
	}

	switch planReservedConcurrency(desired, current) {
	case concurrencyActionPut:
		logger.Info("setting reserved concurrency", "current", formatReservedConcurrency(current), "value", *desired)
		if opt.DryRun {
			return nil
		}
		if _, err := app.lambda.PutFunctionConcurrency(ctx, &lambda.PutFunctionConcurrencyInput{
			FunctionName:                 aws.String(name),
			ReservedConcurrentExecutions: desired,
		}); err != nil {
			return fmt.Errorf("failed to put function concurrency: %w", err)
		}
	case concurrencyActionDelete:
		logger.Info("removing reserved concurrency", "current", formatReservedConcurrency(current))
		if opt.DryRun {
			return nil
		}
		if _, err := app.lambda.DeleteFunctionConcurrency(ctx, &lambda.DeleteFunctionConcurrencyInput{
			FunctionName: aws.String(name),
		}); err != nil {
			return fmt.Errorf("failed to delete function concurrency: %w", err)
		}
	default:
		logger.Info("reserved concurrency is not changed", "value", formatReservedConcurrency(current))
	}
	return nil
}

func formatReservedConcurrency(v *int32) string {
	if v == nil {
		return "(none)"
	}
	return fmt.Sprintf("%d", *v)
}
