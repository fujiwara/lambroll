package lambroll

import (
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
}

// Deploy deployes a new lambda function code
func (app *App) Deploy(opt DeployOption) error {
	var def lambda.CreateFunctionInput
	err := config.LoadWithEnvJSON(&def, *opt.FunctionFilePath)
	if err != nil {
		return errors.Wrap(err, "failed to load "+*opt.FunctionFilePath)
	}

	zipfile, err := CreateZipArchive(*opt.SrcDir, opt.Excludes)
	if err != nil {
		return err
	}
	defer os.Remove(zipfile.Name())

	_, err = app.lambda.GetFunction(&lambda.GetFunctionInput{
		FunctionName: def.FunctionName,
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case lambda.ErrCodeResourceNotFoundException:
				// return app.create(opt, def)
				return nil
			}
		}
		return err
	}

	b, err := ioutil.ReadAll(zipfile)
	if err != nil {
		return errors.Wrap(err, "failed to read zipfile content")
	}

	log.Printf("[info] updating function code %s", *def.FunctionName)
	_, err = app.lambda.UpdateFunctionCode(&lambda.UpdateFunctionCodeInput{
		FunctionName: def.FunctionName,
		Publish:      aws.Bool(true),
		ZipFile:      b,
	})

	return err
}
