package lambroll

import (
	"log"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/pkg/errors"
)

// RollbackOption represents option for Rollback()
type RollbackOption struct {
	FunctionFilePath *string
	DryRun           *bool
	DeleteVersion    *bool
}

func (opt RollbackOption) label() string {
	if *opt.DryRun {
		return "**DRY RUN**"
	}
	return ""
}

// Rollback rollbacks function
func (app *App) Rollback(opt RollbackOption) error {
	def, err := app.loadFunction(*opt.FunctionFilePath)
	if err != nil {
		return errors.Wrap(err, "failed to load function")
	}

	log.Printf("[info] starting rollback function %s", *def.FunctionName)

	res, err := app.lambda.GetAlias(&lambda.GetAliasInput{
		FunctionName: def.FunctionName,
		Name:         aws.String(CurrentAliasName),
	})
	if err != nil {
		return errors.Wrap(err, "failed to get alias")
	}

	currentVersion := *res.FunctionVersion
	cv, err := strconv.ParseInt(currentVersion, 10, 64)
	if err != nil {
		return errors.Wrapf(err, "failed to pase %s as int", currentVersion)
	}

	var prevVersion string
VERSIONS:
	for v := cv - 1; v > 0; v-- {
		log.Printf("[debug] get function version %d", v)
		vs := strconv.FormatInt(v, 10)
		res, err := app.lambda.GetFunction(&lambda.GetFunctionInput{
			FunctionName: def.FunctionName,
			Qualifier:    aws.String(vs),
		})
		if err != nil {
			if aerr, ok := err.(awserr.Error); ok {
				switch aerr.Code() {
				case lambda.ErrCodeResourceNotFoundException:
					log.Printf("[debug] version %s not found", vs)
					continue VERSIONS
				}
			}
			return errors.Wrap(err, "failed to get function")
		}
		prevVersion = *res.Configuration.Version
		break
	}
	if prevVersion == "" {
		return errors.New("unable to detect previous version of function")
	}

	log.Printf("[info] rollbacking function version %s to %s %s", currentVersion, prevVersion, opt.label())
	if *opt.DryRun {
		return nil
	}
	err = app.updateAliases(*def.FunctionName, versionAlias{Version: prevVersion, Name: CurrentAliasName})
	if err != nil {
		return err
	}

	if !*opt.DeleteVersion {
		return nil
	}

	return app.deleteFunctionVersion(*def.FunctionName, currentVersion)
}

func (app *App) deleteFunctionVersion(functionName, version string) error {
	for {
		log.Printf("[debug] checking aliased version")
		res, err := app.lambda.GetAlias(&lambda.GetAliasInput{
			FunctionName: aws.String(functionName),
			Name:         aws.String(CurrentAliasName),
		})
		if err != nil {
			return errors.Wrap(err, "failed to get alias")
		}
		if *res.FunctionVersion == version {
			log.Printf("[debug] version %s still has alias %s, retrying", version, CurrentAliasName)
			time.Sleep(time.Second)
			continue
		}
		break
	}
	log.Printf("[info] deleting function version %s", version)
	_, err := app.lambda.DeleteFunction(&lambda.DeleteFunctionInput{
		FunctionName: aws.String(functionName),
		Qualifier:    aws.String(version),
	})
	if err != nil {
		return errors.Wrap(err, "failed to delete version")
	}
	return nil
}
