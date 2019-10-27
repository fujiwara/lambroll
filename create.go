package lambroll

import (
	"io/ioutil"
	"log"
	"os"

	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/pkg/errors"
)

func (app *App) create(opt DeployOption, def *lambda.CreateFunctionInput) error {
	zipfile, _, err := CreateZipArchive(*opt.SrcDir, opt.Excludes)
	if err != nil {
		return err
	}
	defer os.Remove(zipfile.Name())

	b, err := ioutil.ReadAll(zipfile)
	if err != nil {
		return errors.Wrap(err, "failed to read zipfile content")
	}
	if def.Code == nil {
		def.Code = &lambda.FunctionCode{ZipFile: b}
	}

	log.Println("[info] creating function")
	_, err = app.lambda.CreateFunction(def)
	if err != nil {
		return errors.Wrap(err, "failed to create function")
	}
	return nil
}
