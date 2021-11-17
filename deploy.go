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
	ctx := context.Background()
	if err := (&opt).Expand(); err != nil {
		return errors.Wrap(err, "failed to validate deploy options")
	}
	log.Printf("[debug] %s", opt.String())

	fn, err := app.loadFunction(*opt.FunctionFilePath)
	if err != nil {
		return errors.Wrap(err, "failed to load function")
	}

	log.Printf("[info] starting deploy function %s", *fn.FunctionName)
	var current *lambda.FunctionConfiguration
	if res, err := app.lambda.GetFunction(&lambda.GetFunctionInput{
		FunctionName: fn.FunctionName,
	}); err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case lambda.ErrCodeResourceNotFoundException:
				return app.create(opt, fn)
			}
		}
		return err
	} else {
		current = res.Configuration
	}

	if err := app.prepareFunctionCodeForDeploy(opt, fn); err != nil {
		return errors.Wrap(err, "failed to prepare function code for deploy")
	}

	log.Println("[info] updating function configuration", opt.label())
	confIn := &lambda.UpdateFunctionConfigurationInput{
		DeadLetterConfig:  fn.DeadLetterConfig,
		Description:       fn.Description,
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
	newArch, _ := marshalJSON(fn.Architectures)
	currArch, _ := marshalJSON(current.Architectures)
	if len(newArch) != 0 && !bytes.Equal(currArch, newArch) {
		log.Printf("[warn] Architectures cannot be updated %s %s", currArch, newArch)
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
	retrier := retryPolicy.Start(ctx)
	for retrier.Continue() {
		if _, err := app.lambda.UpdateFunctionConfigurationWithContext(ctx, in); err != nil {
			log.Println("[warn] failed to update function configuration", err)
			continue
		}
		res, err := app.lambda.GetFunction(&lambda.GetFunctionInput{
			FunctionName: in.FunctionName,
		})
		if err != nil {
			log.Println("[warn] failed to get function, retrying", err)
			continue
		} else {
			s := aws.StringValue(res.Configuration.LastUpdateStatus)
			if s == lambda.LastUpdateStatusSuccessful {
				log.Printf("[info] LastUpdateStatus %s", s)
				return res.Configuration, nil
			}
			log.Printf("[debug] LastUpdateStatus %s, retrying", s)
		}
	}
	return nil, errors.New("failed to update function configuration (max retries reached)")
}

func (app *App) updateFunctionCode(ctx context.Context, in *lambda.UpdateFunctionCodeInput) (*lambda.FunctionConfiguration, error) {
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
