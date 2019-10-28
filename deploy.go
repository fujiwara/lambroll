package lambroll

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/kayac/go-config"
	"github.com/pkg/errors"
)

// DeployOption represens an option for Deploy()
type DeployOption struct {
	FunctionFilePath *string
	SrcDir           *string
	Excludes         []string
	ExcludeFile      *string
	DryRun           *bool
}

func (opt DeployOption) label() string {
	if *opt.DryRun {
		return "**DRY RUN**"
	}
	return ""
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

// Deploy deployes a new lambda function code
func (app *App) Deploy(opt DeployOption) error {
	if err := (&opt).Expand(); err != nil {
		return errors.Wrap(err, "failed to validate deploy options")
	}
	log.Printf("[debug] %#v", opt)

	var def lambda.CreateFunctionInput
	err := config.LoadWithEnvJSON(&def, *opt.FunctionFilePath)
	if err != nil {
		return errors.Wrap(err, "failed to load "+*opt.FunctionFilePath)
	}
	log.Printf("[info] starting deploy function %s", *def.FunctionName)

	_, err = app.lambda.GetFunction(&lambda.GetFunctionInput{
		FunctionName: def.FunctionName,
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case lambda.ErrCodeResourceNotFoundException:
				return app.create(opt, &def)
			}
		}
		return err
	}

	err = app.prepareFunctionCodeForDeploy(opt, &def)
	if err != nil {
		return errors.Wrap(err, "failed to prepare function code for deploy")
	}

	log.Println("[info] updating function configuration", opt.label())
	confIn := &lambda.UpdateFunctionConfigurationInput{
		DeadLetterConfig: def.DeadLetterConfig,
		Description:      def.Description,
		Environment:      def.Environment,
		FunctionName:     def.FunctionName,
		Handler:          def.Handler,
		KMSKeyArn:        def.KMSKeyArn,
		Layers:           def.Layers,
		MemorySize:       def.MemorySize,
		Role:             def.Role,
		Runtime:          def.Runtime,
		Timeout:          def.Timeout,
		TracingConfig:    def.TracingConfig,
		VpcConfig:        def.VpcConfig,
	}
	log.Printf("[debug]\n%s", confIn.String())
	if !*opt.DryRun {
		_, err = app.lambda.UpdateFunctionConfiguration(confIn)
		if err != nil {
			return errors.Wrap(err, "failed to update function confugration")
		}
	}

	log.Println("[info] updating function code", opt.label())
	codeIn := &lambda.UpdateFunctionCodeInput{
		FunctionName:    def.FunctionName,
		ZipFile:         def.Code.ZipFile,
		S3Bucket:        def.Code.S3Bucket,
		S3Key:           def.Code.S3Key,
		S3ObjectVersion: def.Code.S3ObjectVersion,
	}
	if *opt.DryRun {
		codeIn.DryRun = aws.Bool(true)
	} else {
		codeIn.Publish = aws.Bool(true)
	}
	log.Printf("[debug]\n%s", codeIn.String())

	res, err := app.lambda.UpdateFunctionCode(codeIn)
	if err != nil {
		return errors.Wrap(err, "failed to update function code")
	}
	if res.Version != nil {
		log.Printf("[info] deployed version %s", *res.Version)
	}
	if *opt.DryRun {
		return nil
	}

	log.Printf("[info] updating alias set %s to version %s", DefaultAliasName, *res.Version)
	alias, err := app.lambda.UpdateAlias(&lambda.UpdateAliasInput{
		FunctionName:    def.FunctionName,
		FunctionVersion: res.Version,
		Name:            aws.String(DefaultAliasName),
	})
	if err != nil {
		return errors.Wrapf(err, "failed to update alias to version", *res.Version)
	}
	log.Println("[info] alias updated")
	log.Printf("[debug]\n%s", alias.String())

	return nil
}
