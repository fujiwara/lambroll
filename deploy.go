package lambroll

import (
	"bytes"
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

// Deploy deployes a new lambda function code
func (app *App) Deploy(opt DeployOption) error {
	if err := (&opt).Expand(); err != nil {
		return errors.Wrap(err, "failed to validate deploy options")
	}
	log.Printf("[debug] %#v", opt)

	def, err := app.loadFunction(*opt.FunctionFilePath)
	if err != nil {
		return errors.Wrap(err, "failed to load function")
	}

	log.Printf("[info] starting deploy function %s", *def.FunctionName)
	_, err = app.lambda.GetFunction(&lambda.GetFunctionInput{
		FunctionName: def.FunctionName,
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case lambda.ErrCodeResourceNotFoundException:
				return app.create(opt, def)
			}
		}
		return err
	}

	err = app.prepareFunctionCodeForDeploy(opt, def)
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

	var newerVersion string
	if !*opt.DryRun {
		_, err := app.lambda.UpdateFunctionConfiguration(confIn)
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
		newerVersion = *res.Version
		log.Printf("[info] deployed version %s", *res.Version)
	}
	if *opt.DryRun {
		return nil
	}

	return app.updateAliases(*def.FunctionName, versionAlias{newerVersion, CurrentAliasName})
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
			return errors.Wrapf(err, "failed to update alias")
		}
		log.Println("[info] alias updated")
		log.Printf("[debug]\n%s", alias.String())
	}
	return nil
}
