package lambroll

import (
	"io"
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/fujiwara/lambroll/appspec"
)

func (app *App) generateAppSpec(funcName string, vs versionAlias) error {
	res, err := app.lambda.GetAlias(&lambda.GetAliasInput{
		FunctionName: aws.String(funcName),
		Name:         aws.String(vs.Name),
	})
	if err != nil {
		return err
	}
	spec := appspec.New(funcName, vs.Name, *res.FunctionVersion, vs.Version)
	log.Println("[info] creating appspec.yml")
	f, err := os.OpenFile("appspec.yml", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.WriteString(f, spec.String())
	return err
}
