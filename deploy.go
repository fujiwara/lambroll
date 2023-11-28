package lambroll

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

// DeployOption represens an option for Deploy()
type DeployOption struct {
	Src           string `help:"function zip archive or src dir" default:"."`
	Publish       bool   `help:"publish function" default:"true"`
	AliasName     string `help:"alias name for publish" default:"current"`
	AliasToLatest bool   `help:"set alias to unpublished $LATEST version" default:"false"`
	DryRun        bool   `help:"dry run" default:"false"`
	SkipArchive   bool   `help:"skip to create zip archive. requires Code.S3Bucket and Code.S3Key in function definition" default:"false"`
	KeepVersions  int    `help:"Number of latest versions to keep. Older versions will be deleted. (Optional value: default 0)." default:"0"`
	FunctionURL   string `help:"path to function-url definiton" default:""`

	ExcludeFileOption
}

func (opt DeployOption) label() string {
	if opt.DryRun {
		return "**DRY RUN**"
	}
	return ""
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

// Deploy deployes a new lambda function code
func (app *App) Deploy(ctx context.Context, opt *DeployOption) error {
	if err := opt.Expand(); err != nil {
		return err
	}
	log.Printf("[debug] %s", opt.String())

	fn, err := app.loadFunction(app.functionFilePath)
	if err != nil {
		return fmt.Errorf("failed to load function: %w", err)
	}

	deployFunctionURL := func(context.Context) error { return nil }
	if opt.FunctionURL != "" {
		fc, err := app.loadFunctionUrl(opt.FunctionURL, *fn.FunctionName)
		if err != nil {
			return fmt.Errorf("failed to load function url config: %w", err)
		}
		deployFunctionURL = func(ctx context.Context) error {
			return app.deployFunctionURL(ctx, fc)
		}
	}

	log.Printf("[info] starting deploy function %s", *fn.FunctionName)
	if current, err := app.lambda.GetFunction(ctx, &lambda.GetFunctionInput{
		FunctionName: fn.FunctionName,
	}); err != nil {
		var nfe *types.ResourceNotFoundException
		if errors.As(err, &nfe) {
			return app.create(ctx, opt, fn)
		}
		return err
	} else if err := validateUpdateFunction(current.Configuration, current.Code, fn); err != nil {
		return err
	}
	fillDefaultValues(fn)

	if err := app.prepareFunctionCodeForDeploy(ctx, opt, fn); err != nil {
		return fmt.Errorf("failed to prepare function code for deploy: %w", err)
	}

	log.Println("[info] updating function configuration", opt.label())
	confIn := &lambda.UpdateFunctionConfigurationInput{
		DeadLetterConfig:  fn.DeadLetterConfig,
		Description:       fn.Description,
		EphemeralStorage:  fn.EphemeralStorage,
		FunctionName:      fn.FunctionName,
		FileSystemConfigs: fn.FileSystemConfigs,
		Handler:           fn.Handler,
		KMSKeyArn:         fn.KMSKeyArn,
		Layers:            fn.Layers,
		LoggingConfig:     fn.LoggingConfig,
		MemorySize:        fn.MemorySize,
		Role:              fn.Role,
		Runtime:           fn.Runtime,
		Timeout:           fn.Timeout,
		TracingConfig:     fn.TracingConfig,
		VpcConfig:         fn.VpcConfig,
		ImageConfig:       fn.ImageConfig,
		SnapStart:         fn.SnapStart,
	}
	if env := fn.Environment; env == nil || env.Variables == nil {
		confIn.Environment = &types.Environment{
			Variables: map[string]string{}, // set empty variables explicitly
		}
	} else {
		confIn.Environment = env
	}

	log.Printf("[debug]\n%s", ToJSONString(confIn))

	var newerVersion string
	if !opt.DryRun {
		proc := func(ctx context.Context) error {
			return app.updateFunctionConfiguration(ctx, confIn)
		}
		if err := app.ensureLastUpdateStatusSuccessful(ctx, *fn.FunctionName, "updating function configuration", proc); err != nil {
			return fmt.Errorf("failed to update function configuration: %w", err)
		}
	}
	if err := app.updateTags(ctx, fn, opt); err != nil {
		return err
	}

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
	if err := app.ensureLastUpdateStatusSuccessful(ctx, *fn.FunctionName, "updating function code", proc); err != nil {
		return err
	}
	if res.Version != nil {
		newerVersion = *res.Version
		log.Printf("[info] deployed version %s %s", *res.Version, opt.label())
	} else {
		newerVersion = versionLatest
		log.Printf("[info] deployed version %s %s", newerVersion, opt.label())
	}
	if opt.DryRun {
		return nil
	}
	if opt.Publish || opt.AliasToLatest {
		err := app.updateAliases(ctx, *fn.FunctionName, versionAlias{newerVersion, opt.AliasName})
		if err != nil {
			return err
		}
	}
	if opt.KeepVersions > 0 { // Ignore zero-value.
		return app.deleteVersions(ctx, *fn.FunctionName, opt.KeepVersions)
	}

	if err := deployFunctionURL(ctx); err != nil {
		return err
	}

	return nil
}

func (app *App) updateFunctionConfiguration(ctx context.Context, in *lambda.UpdateFunctionConfigurationInput) error {
	retrier := retryPolicy.Start(ctx)
	for retrier.Continue() {
		_, err := app.lambda.UpdateFunctionConfiguration(ctx, in)
		if err != nil {
			var rce *types.ResourceConflictException
			if errors.As(err, &rce) {
				log.Println("[debug] retrying", rce.Error())
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
	retrier := retryPolicy.Start(ctx)
	for retrier.Continue() {
		var err error
		res, err = app.lambda.UpdateFunctionCode(ctx, in)
		if err != nil {
			var rce *types.ResourceConflictException
			if errors.As(err, &rce) {
				log.Println("[debug] retrying", err)
				continue
			}
			return nil, fmt.Errorf("failed to update function code: %w", err)
		}
		break
	}
	return res, nil
}

func (app *App) ensureLastUpdateStatusSuccessful(ctx context.Context, name string, msg string, code func(ctx context.Context) error) error {
	log.Println("[info]", msg, "...")
	if err := app.waitForLastUpdateStatusSuccessful(ctx, name); err != nil {
		return err
	}
	if err := code(ctx); err != nil {
		return err
	}
	log.Println("[info]", msg, "accepted. waiting for LastUpdateStatus to be successful.")
	if err := app.waitForLastUpdateStatusSuccessful(ctx, name); err != nil {
		return err
	}
	log.Println("[info]", msg, "successfully")
	return nil
}

func (app *App) waitForLastUpdateStatusSuccessful(ctx context.Context, name string) error {
	retrier := retryPolicy.Start(ctx)
	for retrier.Continue() {
		res, err := app.lambda.GetFunction(ctx, &lambda.GetFunctionInput{
			FunctionName: aws.String(name),
		})
		if err != nil {
			log.Println("[warn] failed to get function, retrying", err)
			continue
		} else {
			state := res.Configuration.State
			last := res.Configuration.LastUpdateStatus
			log.Printf("[info] State:%s LastUpdateStatus:%s", state, last)
			if last == types.LastUpdateStatusSuccessful {
				return nil
			}
			log.Printf("[info] waiting for LastUpdateStatus %s", types.LastUpdateStatusSuccessful)
		}
	}
	return fmt.Errorf("max retries reached")
}

func (app *App) updateAliases(ctx context.Context, functionName string, vs ...versionAlias) error {
	for _, v := range vs {
		log.Printf("[info] updating alias set %s to version %s", v.Name, v.Version)
		_, err := app.lambda.UpdateAlias(ctx, &lambda.UpdateAliasInput{
			FunctionName:    aws.String(functionName),
			FunctionVersion: aws.String(v.Version),
			Name:            aws.String(v.Name),
		})
		if err != nil {
			var nfe *types.ResourceNotFoundException
			if errors.As(err, &nfe) {
				log.Printf("[info] alias %s is not found. creating alias", v.Name)
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
		log.Println("[info] alias updated")
	}
	return nil
}

func (app *App) deleteVersions(ctx context.Context, functionName string, keepVersions int) error {
	if keepVersions <= 0 {
		log.Printf("[info] specify --keep-versions")
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

		log.Printf("[info] deleting function version: %s", *v.Version)
		_, err := app.lambda.DeleteFunction(ctx, &lambda.DeleteFunctionInput{
			FunctionName: aws.String(functionName),
			Qualifier:    v.Version,
		})
		if err != nil {
			return fmt.Errorf("failed to delete version: %w", err)
		}
	}

	log.Printf("[info] except %d latest versions are deleted", keepVersions)
	return nil
}
