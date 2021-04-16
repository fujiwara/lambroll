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
	DryRun           *bool
	SkipArchive      *bool
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
func (opt *DeployOption) Expand() error {
	if opt.ExcludeFile == nil {
		return nil
	}
	b, err := ioutil.ReadFile(*opt.ExcludeFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	lines := bytes.Split(b, []byte{'\n'})
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 || bytes.HasPrefix(line, []byte{'#'}) {
			// skip blank or comment line
			continue
		}
		opt.Excludes = append(opt.Excludes, string(line))
	}
	return nil
}

func (opt *DeployOption) String() string {
	b, _ := json.Marshal(opt)
	return string(b)
}

// Deploy deployes a new lambda function code
func (app *App) Deploy(opt DeployOption) error {
	if err := (&opt).Expand(); err != nil {
		return errors.Wrap(err, "failed to validate deploy options")
	}
	log.Printf("[debug] %s", opt.String())

	fn, err := app.loadFunction(*opt.FunctionFilePath)
	if err != nil {
		return errors.Wrap(err, "failed to load function")
	}

	log.Printf("[info] starting deploy function %s", *fn.FunctionName)
	_, err = app.lambda.GetFunction(&lambda.GetFunctionInput{
		FunctionName: fn.FunctionName,
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case lambda.ErrCodeResourceNotFoundException:
				return app.create(opt, fn)
			}
		}
		return err
	}

	err = app.prepareFunctionCodeForDeploy(opt, fn)
	if err != nil {
		return errors.Wrap(err, "failed to prepare function code for deploy")
	}

	log.Println("[info] updating function configuration", opt.label())
	confIn := &lambda.UpdateFunctionConfigurationInput{
		DeadLetterConfig:  fn.DeadLetterConfig,
		Description:       fn.Description,
		Environment:       fn.Environment,
		FunctionName:      fn.FunctionName,
		FileSystemConfigs: fn.FileSystemConfigs,
		Handler:           fn.Handler,
		KMSKeyArn:         fn.KMSKeyArn,
		Layers:            fn.Layers,
		MemorySize:        fn.MemorySize,
		Role:              fn.Role,
		Runtime:           fn.Runtime,
		Timeout:           fn.Timeout,
		TracingConfig:     fn.TracingConfig,
		VpcConfig:         fn.VpcConfig,
		ImageConfig:       fn.ImageConfig,
	}
	log.Printf("[debug]\n%s", confIn.String())

	var newerVersion string
	if !*opt.DryRun {
		if _, err := app.lambda.UpdateFunctionConfiguration(confIn); err != nil {
			return errors.Wrap(err, "failed to update function configuration")
		}
	}
	if err := app.updateTags(fn, opt); err != nil {
		return err
	}

	log.Println("[info] updating function code", opt.label())
	codeIn := &lambda.UpdateFunctionCodeInput{
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

	res, err := app.updateFunctionCodeWithRetry(context.Background(), codeIn)
	if err != nil {
		return err
	}
	if res.Version != nil {
		newerVersion = *res.Version
		log.Printf("[info] deployed version %s %s", *res.Version, opt.label())
	} else {
		log.Println("[info] deployed")
	}
	if *opt.DryRun || !*opt.Publish {
		return nil
	}

	return app.updateAliases(*fn.FunctionName, versionAlias{newerVersion, *opt.AliasName})
}

func (app *App) updateFunctionCodeWithRetry(ctx context.Context, in *lambda.UpdateFunctionCodeInput) (*lambda.FunctionConfiguration, error) {
	retrier := retryPolicy.Start(ctx)
	for retrier.Continue() {
		res, err := app.lambda.UpdateFunctionCode(in)
		if err != nil {
			if aerr, ok := err.(awserr.Error); ok {
				switch aerr.Code() {
				case lambda.ErrCodeResourceConflictException:
					log.Println("[debug] retrying", aerr.Message())
					continue
				}
			} else {
				return nil, errors.Wrap(err, "failed to update function code")
			}
		}
		return res, nil
	}
	return nil, errors.New("failed to update function code (max retries)")
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
