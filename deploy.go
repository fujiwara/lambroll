package lambroll

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/aereal/jsondiff"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/itchyny/gojq"
)

// DeployOption represents an option for Deploy()
type DeployOption struct {
	Src               string `help:"function zip archive or src dir" default:"."`
	Publish           bool   `help:"publish function" default:"true" negatable:""`
	AliasName         string `name:"alias" help:"alias name for publish" default:"current"`
	AliasToLatest     bool   `help:"set alias to unpublished $LATEST version" default:"false"`
	SkipArchive       bool   `help:"skip to create zip archive. requires Code.S3Bucket and Code.S3Key in function definition" default:"false"`
	KeepVersions      int    `help:"Number of latest versions to keep. Older versions will be deleted. (Optional value: default 0)." default:"0"`
	Ignore            string `help:"ignore fields by jq queries in function.json" default:""`
	FunctionURL       string `help:"path to function-url definition" default:"" env:"LAMBROLL_FUNCTION_URL"`
	SkipConfiguration bool   `help:"skip updating function configuration, deploy function code and aliases only" default:"false"`
	SkipFunction      bool   `help:"skip to deploy a function. deploy function-url only" default:"false"`

	DryRunOption
	ZipOption
}

type versionAlias struct {
	Version string
	Name    string
}

// Expand expands ExcludeFile contents to Excludes
func expandExcludeFile(file string) ([]string, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	lines := bytes.Split(b, []byte{'\n'})
	excludes := make([]string, 0, len(lines))
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 || bytes.HasPrefix(line, []byte{'#'}) {
			// skip blank or comment line
			continue
		}
		excludes = append(excludes, string(line))
	}
	return excludes, nil
}

func (opt *DeployOption) String() string {
	b, _ := json.Marshal(opt)
	return string(b)
}

// Deploy deploys a new lambda function code
func (app *App) Deploy(ctx context.Context, opt *DeployOption) error {
	logger := opt.logger()

	if err := opt.Expand(); err != nil {
		return err
	}
	logger.Debug("deploy options", "options", opt.String())

	fn, err := app.loadFunction(app.functionFilePath)
	if err != nil {
		return fmt.Errorf("failed to load function: %w", err)
	}

	deployFunctionURL := func(context.Context) error { return nil }
	if opt.FunctionURL != "" {
		deployFunctionURL = func(ctx context.Context) error {
			fc, err := app.loadFunctionUrl(opt.FunctionURL, *fn.FunctionName)
			if err != nil {
				return fmt.Errorf("failed to load function url config: %w", err)
			}
			return app.deployFunctionURL(ctx, fc, opt)
		}
	}

	if opt.SkipFunction {
		// skip to deploy a function. deploy function-url only
		return deployFunctionURL(ctx)
	}

	logger.Info("starting deploy function", "function", *fn.FunctionName)
	if current, err := app.lambda.GetFunction(ctx, &lambda.GetFunctionInput{
		FunctionName: fn.FunctionName,
	}); err != nil {
		var nfe *types.ResourceNotFoundException
		if !errors.As(err, &nfe) {
			return err
		}
		if err := app.create(ctx, opt, fn); err != nil {
			return err
		}
		if err := deployFunctionURL(ctx); err != nil {
			return err
		}
		return nil
	} else if err := validateUpdateFunction(current.Configuration, current.Code, fn); err != nil {
		return err
	}
	fillDefaultValues(fn)

	if err := app.prepareFunctionCodeForDeploy(ctx, opt, fn); err != nil {
		return fmt.Errorf("failed to prepare function code for deploy: %w", err)
	}

	if ignore := opt.Ignore; ignore != "" {
		q, err := gojq.Parse(ignore)
		if err != nil {
			return fmt.Errorf("failed to parse ignore query: %w", err)
		}
		// Use the del() function to ignore a given path
		q = &gojq.Query{
			Term: &gojq.Term{
				Type: gojq.TermTypeFunc,
				Func: &gojq.Func{
					Name: "del",
					Args: []*gojq.Query{q},
				},
			},
		}
		fnAny, _ := marshalAny(fn)
		fnAny, err = jsondiff.ModifyValue(q, fnAny)
		if err != nil {
			return fmt.Errorf("failed to modify function: %w", err)
		}
		src, _ := json.Marshal(fnAny)
		fn = &Function{}
		unmarshalJSON(src, &fn, app.functionFilePath)
	}

	// update function configuration
	if opt.SkipConfiguration {
		logger.Info("skip to deploy function configuration")
	} else {
		if err := app.deployFunctionConfiguration(ctx, fn, opt); err != nil {
			return fmt.Errorf("failed to deploy function configuration: %w", err)
		}
	}

	// update function code
	newerVersion, err := app.deployFunctionCode(ctx, fn, opt)
	if err != nil {
		return fmt.Errorf("failed to deploy function code: %w", err)
	}

	if opt.DryRun {
		return nil
	}

	// publish function only for managed instance
	if opt.Publish && isForManagedInstance(fn) {
		if err := app.publishFunction(ctx, *fn.FunctionName); err != nil {
			return fmt.Errorf("failed to publish function: %w", err)
		}
	}

	// update function aliases
	if opt.Publish || opt.AliasToLatest {
		err := app.updateAliases(ctx, *fn.FunctionName, versionAlias{newerVersion, opt.AliasName})
		if err != nil {
			return err
		}
	}
	if opt.KeepVersions > 0 { // Ignore zero-value.
		if err := app.deleteVersions(ctx, *fn.FunctionName, opt.KeepVersions); err != nil {
			return err
		}
	}

	// deploy function-url
	if err := deployFunctionURL(ctx); err != nil {
		return err
	}

	return nil
}

func (app *App) deployFunctionConfiguration(ctx context.Context, fn *Function, opt *DeployOption) error {
	logger := opt.logger()

	logger.Info("updating function configuration")
	confIn := &lambda.UpdateFunctionConfigurationInput{
		DeadLetterConfig:  fn.DeadLetterConfig,
		Description:       fn.Description,
		Environment:       fn.Environment,
		EphemeralStorage:  fn.EphemeralStorage,
		FunctionName:      fn.FunctionName,
		FileSystemConfigs: fn.FileSystemConfigs,
		Handler:           fn.Handler,
		KMSKeyArn:         fn.KMSKeyArn,
		Layers:            fn.Layers,
		LoggingConfig:     fn.LoggingConfig,
		DurableConfig:     fn.DurableConfig,
		MemorySize:        fn.MemorySize,
		Role:              fn.Role,
		Runtime:           fn.Runtime,
		Timeout:           fn.Timeout,
		TracingConfig:     fn.TracingConfig,
		VpcConfig:         fn.VpcConfig,
		ImageConfig:       fn.ImageConfig,
		SnapStart:         fn.SnapStart,
	}
	slog.Debug("update function configuration input", "input", jsonStr(confIn))

	if !opt.DryRun {
		proc := func(ctx context.Context) error {
			return app.updateFunctionConfiguration(ctx, confIn)
		}
		if err := app.ensureLastUpdateStatusSuccessful(ctx, *fn.FunctionName, "updating function configuration", proc, logger); err != nil {
			return fmt.Errorf("failed to update function configuration: %w", err)
		}
	}
	if err := app.updateTags(ctx, fn, opt); err != nil {
		return err
	}
	return nil
}

func (app *App) deployFunctionCode(ctx context.Context, fn *Function, opt *DeployOption) (string, error) {
	logger := opt.logger()

	codeIn := &lambda.UpdateFunctionCodeInput{
		Architectures:   fn.Architectures,
		FunctionName:    fn.FunctionName,
		ZipFile:         fn.Code.ZipFile,
		S3Bucket:        fn.Code.S3Bucket,
		S3Key:           fn.Code.S3Key,
		S3ObjectVersion: fn.Code.S3ObjectVersion,
		ImageUri:        fn.Code.ImageUri,
	}
	if opt.DryRun {
		codeIn.DryRun = true
	} else {
		codeIn.Publish = opt.Publish
	}

	var res *lambda.UpdateFunctionCodeOutput
	proc := func(ctx context.Context) error {
		var err error
		// set res outside of this function
		res, err = app.updateFunctionCode(ctx, codeIn)
		return err
	}
	if err := app.ensureLastUpdateStatusSuccessful(ctx, *fn.FunctionName, "updating function code", proc, logger); err != nil {
		return "", err
	}
	var newerVersion string
	if res.Version != nil {
		newerVersion = *res.Version
		logger.Info("deployed version", "version", *res.Version)
	} else {
		newerVersion = versionLatest
		logger.Info("deployed version", "version", newerVersion)
	}
	return newerVersion, nil
}

func (app *App) updateFunctionConfiguration(ctx context.Context, in *lambda.UpdateFunctionConfigurationInput) error {
	retryer := retryPolicy.Start(ctx)
	for retryer.Continue() {
		_, err := app.lambda.UpdateFunctionConfiguration(ctx, in)
		if err != nil {
			var rce *types.ResourceConflictException
			if errors.As(err, &rce) {
				slog.Debug("retrying", "error", rce.Error())
				continue
			}
			return fmt.Errorf("failed to update function configuration: %w", err)
		}
		return nil
	}
	return fmt.Errorf("failed to update function configuration (max retries reached)")
}

func (app *App) updateFunctionCode(ctx context.Context, in *lambda.UpdateFunctionCodeInput) (*lambda.UpdateFunctionCodeOutput, error) {
	var res *lambda.UpdateFunctionCodeOutput
	retryer := retryPolicy.Start(ctx)
	for retryer.Continue() {
		var err error
		res, err = app.lambda.UpdateFunctionCode(ctx, in)
		if err != nil {
			var rce *types.ResourceConflictException
			if errors.As(err, &rce) {
				slog.Debug("retrying", "error", err)
				continue
			}
			return nil, fmt.Errorf("failed to update function code: %w", err)
		}
		break
	}
	return res, nil
}

func (app *App) ensureLastUpdateStatusSuccessful(ctx context.Context, name string, msg string, code func(ctx context.Context) error, logger *slog.Logger) error {
	logger.Info(msg + " ...")
	if err := app.waitForLastUpdateStatusSuccessful(ctx, name); err != nil {
		return err
	}
	if err := code(ctx); err != nil {
		return err
	}
	logger.Info(msg + " accepted. waiting for LastUpdateStatus to be successful.")
	if err := app.waitForLastUpdateStatusSuccessful(ctx, name); err != nil {
		return err
	}
	logger.Info(msg + " successfully")
	return nil
}

func (app *App) waitForLastUpdateStatusSuccessful(ctx context.Context, name string) error {
	retryer := retryPolicy.Start(ctx)
	for retryer.Continue() {
		res, err := app.lambda.GetFunction(ctx, &lambda.GetFunctionInput{
			FunctionName: aws.String(name),
		})
		if err != nil {
			slog.Warn("failed to get function, retrying", "error", err)
			continue
		} else {
			state := res.Configuration.State
			last := res.Configuration.LastUpdateStatus
			slog.Info("function status", "state", state, "lastUpdateStatus", last)
			if last == types.LastUpdateStatusSuccessful {
				return nil
			}
			slog.Info("waiting for LastUpdateStatus", "status", types.LastUpdateStatusSuccessful)
		}
	}
	return fmt.Errorf("max retries reached")
}

func (app *App) publishFunction(ctx context.Context, functionName string) error {
	slog.Info("publishing function", "function", functionName, "to", types.FunctionVersionLatestPublishedLatestPublished)
	_, err := app.lambda.PublishVersion(ctx, &lambda.PublishVersionInput{
		FunctionName: aws.String(functionName),
		PublishTo:    types.FunctionVersionLatestPublishedLatestPublished,
	})
	if err != nil {
		return fmt.Errorf("failed to publish function: %w", err)
	}
	slog.Info("function published")
	return nil
}

func (app *App) updateAliases(ctx context.Context, functionName string, vs ...versionAlias) error {
	for _, v := range vs {
		slog.Info("updating alias", "name", v.Name, "version", v.Version)
		_, err := app.lambda.UpdateAlias(ctx, &lambda.UpdateAliasInput{
			FunctionName:    aws.String(functionName),
			FunctionVersion: aws.String(v.Version),
			Name:            aws.String(v.Name),
		})
		if err != nil {
			var nfe *types.ResourceNotFoundException
			if errors.As(err, &nfe) {
				slog.Info("alias not found. creating alias", "name", v.Name)
				_, err := app.lambda.CreateAlias(ctx, &lambda.CreateAliasInput{
					FunctionName:    aws.String(functionName),
					FunctionVersion: aws.String(v.Version),
					Name:            aws.String(v.Name),
				})
				if err != nil {
					return fmt.Errorf("failed to create alias: %w", err)
				}
			} else {
				return fmt.Errorf("failed to update alias: %w", err)
			}
		}
		slog.Info("alias updated")
	}
	return nil
}

func (app *App) deleteVersions(ctx context.Context, functionName string, keepVersions int) error {
	if keepVersions <= 0 {
		slog.Info("specify --keep-versions")
		return nil
	}

	params := &lambda.ListVersionsByFunctionInput{
		FunctionName: aws.String(functionName),
	}

	// versions will be set asc order, like 1 to N
	versions := []types.FunctionConfiguration{}
	for {
		res, err := app.lambda.ListVersionsByFunction(ctx, params)
		if err != nil {
			return fmt.Errorf("failed to list versions: %w", err)
		}
		versions = append(versions, res.Versions...)
		if res.NextMarker != nil {
			params.Marker = res.NextMarker
			continue
		}
		break
	}

	keep := len(versions) - keepVersions
	for i, v := range versions {
		if i == 0 {
			continue
		}
		if i >= keep {
			break
		}

		slog.Info("deleting function version", "version", *v.Version)
		_, err := app.lambda.DeleteFunction(ctx, &lambda.DeleteFunctionInput{
			FunctionName: aws.String(functionName),
			Qualifier:    v.Version,
		})
		if err != nil {
			return fmt.Errorf("failed to delete version: %w", err)
		}
	}

	slog.Info("versions deleted", "kept", keepVersions)
	return nil
}
