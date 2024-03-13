package lambroll

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

// RollbackOption represents option for Rollback()
type RollbackOption struct {
	DryRun        bool   `default:"false" help:"dry run"`
	Alias         string `default:"current" help:"alias to rollback"`
	Version       string `default:"" help:"version to rollback (default: previous version auto detected)"`
	DeleteVersion bool   `default:"false" help:"delete rolled back version"`
}

func (opt RollbackOption) label() string {
	if opt.DryRun {
		return "**DRY RUN**"
	}
	return ""
}

// Rollback rollbacks function
func (app *App) Rollback(ctx context.Context, opt *RollbackOption) error {
	fn, err := app.loadFunction(app.functionFilePath)
	if err != nil {
		return fmt.Errorf("failed to load function: %w", err)
	}

	log.Printf("[info] starting rollback function %s:%s", *fn.FunctionName, opt.Alias)

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

	log.Printf("[info] rollbacking function version %s to %s %s", currentVersion, prevVersion, opt.label())
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
		log.Printf("[debug] get function version %d", v)
		vs := strconv.FormatInt(v, 10)
		res, err := app.lambda.GetFunction(ctx, &lambda.GetFunctionInput{
			FunctionName: aws.String(name),
			Qualifier:    aws.String(vs),
		})
		if err != nil {
			var nfe *types.ResourceNotFoundException
			if errors.As(err, &nfe) {
				log.Printf("[debug] version %s not found", vs)
				continue VERSIONS
			} else {
				return "", fmt.Errorf("failed to get function: %w", err)
			}
		}
		if pv := *res.Configuration.Version; aliases[pv] != nil {
			// skip if the version has alias
			log.Printf("[info] version %s has alias %v, skipping", pv, aliases[pv])
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
	for {
		log.Printf("[debug] checking aliased version")
		res, err := app.lambda.GetAlias(ctx, &lambda.GetAliasInput{
			FunctionName: aws.String(functionName),
			Name:         aws.String(CurrentAliasName),
		})
		if err != nil {
			return fmt.Errorf("failed to get alias: %w", err)
		}
		if *res.FunctionVersion == version {
			log.Printf("[debug] version %s still has alias %s, retrying", version, CurrentAliasName)
			time.Sleep(time.Second)
			continue
		}
		break
	}
	log.Printf("[info] deleting function version %s", version)
	_, err := app.lambda.DeleteFunction(ctx, &lambda.DeleteFunctionInput{
		FunctionName: aws.String(functionName),
		Qualifier:    aws.String(version),
	})
	if err != nil {
		return fmt.Errorf("failed to delete version: %w", err)
	}
	return nil
}
