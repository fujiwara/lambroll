package lambroll

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

// RollbackOption represents option for Rollback()
type RollbackOption struct {
	Alias         string `default:"current" help:"alias to rollback"`
	Version       string `default:"" help:"version to rollback (default: previous version auto detected)"`
	DeleteVersion bool   `default:"false" help:"delete rolled back version"`

	DryRunOption
}

// Rollback rollbacks function
func (app *App) Rollback(ctx context.Context, opt *RollbackOption) error {
	logger := opt.logger()

	fn, err := app.loadFunction(app.functionFilePath)
	if err != nil {
		return fmt.Errorf("failed to load function: %w", err)
	}

	logger.Info("starting rollback function", "function", *fn.FunctionName, "alias", opt.Alias)

	res, err := app.lambda.GetAlias(ctx, &lambda.GetAliasInput{
		FunctionName: fn.FunctionName,
		Name:         aws.String(opt.Alias),
	})
	if err != nil {
		return fmt.Errorf("failed to get alias: %w", err)
	}

	currentVersion := *res.FunctionVersion
	var prevVersion string
	if opt.Version != "" {
		prevVersion = opt.Version
	} else {
		prevVersion, err = app.findPreviousVersion(ctx, *fn.FunctionName, currentVersion)
		if err != nil {
			return fmt.Errorf("failed to find previous version: %w", err)
		}
	}

	logger.Info("rolling back function version", "from", currentVersion, "to", prevVersion)
	if opt.DryRun {
		return nil
	}
	err = app.updateAliases(ctx, *fn.FunctionName, versionAlias{Version: prevVersion, Name: opt.Alias})
	if err != nil {
		return err
	}

	if !opt.DeleteVersion {
		return nil
	}

	return app.deleteFunctionVersion(ctx, *fn.FunctionName, currentVersion)
}

func (app *App) findPreviousVersion(ctx context.Context, name, currentVersion string) (string, error) {
	aliases, err := app.getAliases(ctx, name)
	if err != nil {
		return "", fmt.Errorf("failed to get aliases: %w", err)
	}
	cv, err := strconv.ParseInt(currentVersion, 10, 64)
	if err != nil {
		return "", fmt.Errorf("failed to pase %s as int: %w", currentVersion, err)
	}

	var prevVersion string
VERSIONS:
	for v := cv - 1; v > 0; v-- {
		slog.Debug("get function version", "version", v)
		vs := strconv.FormatInt(v, 10)
		res, err := app.lambda.GetFunction(ctx, &lambda.GetFunctionInput{
			FunctionName: aws.String(name),
			Qualifier:    aws.String(vs),
		})
		if err != nil {
			var nfe *types.ResourceNotFoundException
			if errors.As(err, &nfe) {
				slog.Debug("version not found", "version", vs)
				continue VERSIONS
			} else {
				return "", fmt.Errorf("failed to get function: %w", err)
			}
		}
		if pv := *res.Configuration.Version; aliases[pv] != nil {
			// skip if the version has alias
			slog.Info("version has alias, skipping", "version", pv, "aliases", aliases[pv])
			continue VERSIONS
		}
		prevVersion = *res.Configuration.Version
		break
	}
	if prevVersion == "" {
		return "", fmt.Errorf("unable to detect previous version of function")
	}
	return prevVersion, nil
}

func (app *App) deleteFunctionVersion(ctx context.Context, functionName, version string) error {
	slog.Info("deleting function version", "version", version)
	_, err := app.lambda.DeleteFunction(ctx, &lambda.DeleteFunctionInput{
		FunctionName: aws.String(functionName),
		Qualifier:    aws.String(version),
	})
	if err != nil {
		return fmt.Errorf("failed to delete version: %w", err)
	}
	return nil
}
