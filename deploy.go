package lambroll

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/pkg/errors"
)

// DeployOption represens an option for Deploy()
type DeployOption struct {
	FunctionFilePath *string
	Src              *string
	Excludes         []string
	ExcludeFile      *string
	Publish          *bool
	AliasName        *string
	AliasToLatest    *bool
	DryRun           *bool
	SkipArchive      *bool
	KeepVersions     *int
}

func (opt DeployOption) label() string {
	if *opt.DryRun {
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
	b, err := ioutil.ReadFile(file)
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
func (app *App) Deploy(opt DeployOption) error {
	ctx := context.Background()
	excludes, err := expandExcludeFile(*opt.ExcludeFile)
	if err != nil {
		return errors.Wrap(err, "failed to parse exclude-file")
	}
	opt.Excludes = append(opt.Excludes, excludes...)
	log.Printf("[debug] %s", opt.String())

	fn, err := app.loadFunction(*opt.FunctionFilePath)
	if err != nil {
		return errors.Wrap(err, "failed to load function")
	}

	log.Printf("[info] starting deploy function %s", *fn.FunctionName)
	if current, err := app.lambda.GetFunction(&lambda.GetFunctionInput{
		FunctionName: fn.FunctionName,
	}); err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case lambda.ErrCodeResourceNotFoundException:
				return app.create(opt, fn)
			}
		}
		return err
	} else if err := validateUpdateFunction(current.Configuration, current.Code, fn); err != nil {
		return err
	}
	fillDefaultValues(fn)

	if err := app.prepareFunctionCodeForDeploy(opt, fn); err != nil {
		return errors.Wrap(err, "failed to prepare function code for deploy")
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
		confIn.Environment = &lambda.Environment{
			Variables: map[string]*string{}, // set empty variables explicitly
		}
	} else {
		confIn.Environment = env
	}

	log.Printf("[debug]\n%s", confIn.String())

	var newerVersion string
	if !*opt.DryRun {
		if _, err := app.updateFunctionConfiguration(ctx, confIn); err != nil {
			return errors.Wrap(err, "failed to update function configuration")
		}
	}
	if err := app.updateTags(fn, opt); err != nil {
		return err
	}

	log.Println("[info] updating function code", opt.label())
	codeIn := &lambda.UpdateFunctionCodeInput{
		Architectures:   fn.Architectures,
		FunctionName:    fn.FunctionName,
		ZipFile:         fn.Code.ZipFile,
		S3Bucket:        fn.Code.S3Bucket,
		S3Key:           fn.Code.S3Key,
		S3ObjectVersion: fn.Code.S3ObjectVersion,
		ImageUri:        fn.Code.ImageUri,
	}
	if *opt.DryRun {
		codeIn.DryRun = aws.Bool(true)
	} else {
		codeIn.Publish = opt.Publish
	}
	log.Printf("[debug]\n%s", codeIn.String())

	res, err := app.updateFunctionCode(ctx, codeIn)
	if err != nil {
		return err
	}
	if res.Version != nil {
		newerVersion = *res.Version
		log.Printf("[info] deployed version %s %s", *res.Version, opt.label())
	} else {
		newerVersion = versionLatest
		log.Printf("[info] deployed version %s %s", newerVersion, opt.label())
	}
	if *opt.DryRun {
		return nil
	}
	if *opt.Publish || *opt.AliasToLatest {
		err := app.updateAliases(*fn.FunctionName, versionAlias{newerVersion, *opt.AliasName})
		if err != nil {
			return err
		}
	}
	if *opt.KeepVersions > 0 { // Ignore zero-value.
		return app.deleteVersions(*fn.FunctionName, *opt.KeepVersions)
	}
	return nil
}

func (app *App) updateFunctionConfiguration(ctx context.Context, in *lambda.UpdateFunctionConfigurationInput) (*lambda.FunctionConfiguration, error) {
	if err := app.waitForLastUpdateStatusSuccessful(ctx, *in.FunctionName); err != nil {
		return nil, err
	}

	retrier := retryPolicy.Start(ctx)
	for retrier.Continue() {
		res, err := app.lambda.UpdateFunctionConfigurationWithContext(ctx, in)
		if err != nil {
			if aerr, ok := err.(awserr.Error); ok {
				switch aerr.Code() {
				case lambda.ErrCodeResourceConflictException:
					log.Println("[debug] retrying", aerr.Message())
					continue
				}
			}
			return nil, errors.Wrap(err, "failed to update function configuration")
		}
		log.Println("[info] updated function configuration successfully")
		return res, nil
	}
	return nil, errors.New("failed to update function configuration (max retries reached)")
}

func (app *App) updateFunctionCode(ctx context.Context, in *lambda.UpdateFunctionCodeInput) (*lambda.FunctionConfiguration, error) {
	if err := app.waitForLastUpdateStatusSuccessful(ctx, *in.FunctionName); err != nil {
		return nil, err
	}

	var res *lambda.FunctionConfiguration
	retrier := retryPolicy.Start(ctx)
	for retrier.Continue() {
		var err error
		res, err = app.lambda.UpdateFunctionCodeWithContext(ctx, in)
		if err != nil {
			if aerr, ok := err.(awserr.Error); ok {
				switch aerr.Code() {
				case lambda.ErrCodeResourceConflictException:
					log.Println("[debug] retrying", aerr.Message())
					continue
				}
			}
			return nil, errors.Wrap(err, "failed to update function code")
		}
		log.Println("[info] update function code request was accepted")
		break
	}

	if !retrier.Continue() {
		return nil, errors.New("failed to update function code (max retries reached)")
	}

	if err := app.waitForLastUpdateStatusSuccessful(ctx, *in.FunctionName); err != nil {
		return nil, err
	}
	log.Println("[info] updated function code successfully")

	return res, nil
}

func (app *App) waitForLastUpdateStatusSuccessful(ctx context.Context, name string) error {
	retrier := retryPolicy.Start(ctx)
	for retrier.Continue() {
		res, err := app.lambda.GetFunction(&lambda.GetFunctionInput{
			FunctionName: aws.String(name),
		})
		if err != nil {
			log.Println("[warn] failed to get function, retrying", err)
			continue
		} else {
			state := aws.StringValue(res.Configuration.State)
			last := aws.StringValue(res.Configuration.LastUpdateStatus)
			log.Printf("[info] State:%s LastUpdateStatus:%s", state, last)
			if last == lambda.LastUpdateStatusSuccessful {
				return nil
			}
			log.Printf("[info] waiting for LastUpdateStatus %s", lambda.LastUpdateStatusSuccessful)
		}
	}
	return errors.New("max retries reached")
}

func (app *App) updateAliases(functionName string, vs ...versionAlias) error {
	for _, v := range vs {
		log.Printf("[info] updating alias set %s to version %s", v.Name, v.Version)
		alias, err := app.lambda.UpdateAlias(&lambda.UpdateAliasInput{
			FunctionName:    aws.String(functionName),
			FunctionVersion: aws.String(v.Version),
			Name:            aws.String(v.Name),
		})
		if err != nil {
			if aerr, ok := err.(awserr.Error); ok {
				switch aerr.Code() {
				case lambda.ErrCodeResourceNotFoundException:
					log.Printf("[info] alias %s is not found. creating alias", v.Name)
					alias, err = app.lambda.CreateAlias(&lambda.CreateAliasInput{
						FunctionName:    aws.String(functionName),
						FunctionVersion: aws.String(v.Version),
						Name:            aws.String(v.Name),
					})
				}
			}
			if err != nil {
				return errors.Wrap(err, "failed to update alias")
			}
		}
		log.Println("[info] alias updated")
		log.Printf("[debug]\n%s", alias.String())
	}
	return nil
}

func (app *App) deleteVersions(functionName string, keepVersions int) error {
	if keepVersions <= 0 {
		log.Printf("[info] specify --keep-versions")
		return nil
	}

	params := &lambda.ListVersionsByFunctionInput{
		FunctionName: aws.String(functionName),
	}

	// versions will be set asc order, like 1 to N
	versions := []*lambda.FunctionConfiguration{}

	for {
		req, resp := app.lambda.ListVersionsByFunctionRequest(params)
		if err := req.Send(); err != nil {
			return err
		}

		versions = append(versions, resp.Versions...)

		if resp.NextMarker != nil {
			params.Marker = resp.NextMarker
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
		_, err := app.lambda.DeleteFunction(&lambda.DeleteFunctionInput{
			FunctionName: aws.String(functionName),
			Qualifier:    v.Version,
		})
		if err != nil {
			return errors.Wrap(err, "failed to delete version")
		}
	}

	log.Printf("[info] except %d latest versions are deleted", keepVersions)
	return nil
}
