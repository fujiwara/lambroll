package lambroll

import (
	"encoding/json"
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/pkg/errors"
)

type TFOption struct {
	FunctionFilePath *string
	Deploy           *bool
}

type TFOutput struct {
	FunctionName string `json:"function_name"`
	Arn          string `json:"arn"`
	ID           string `json:"id"`
	Role         string `json:"role"`
	Runtime      string `json:"runtime"`
}

func (app *App) TF(opt TFOption) error {
	newFunc, err := app.loadFunction(*opt.FunctionFilePath)
	if err != nil {
		return errors.Wrap(err, "failed to load function")
	}
	if aws.BoolValue(opt.Deploy) {
		deployOpt := DeployOption{
			FunctionFilePath: opt.FunctionFilePath,
		}
		if err := json.NewDecoder(os.Stdin).Decode(&deployOpt); err != nil {
			return errors.Wrap(err, "failed to decode deploy option from STDIN")
		}
		log.Printf("[info] Deploying function with %v", deployOpt)
		if err := app.Deploy(deployOpt); err != nil {
			return err
		}
	}
	name := *newFunc.FunctionName
	out := TFOutput{
		FunctionName: name,
		Arn:          app.functionArn(*newFunc.FunctionName),
		ID:           name,
		Role:         *newFunc.Role,
		Runtime:      *newFunc.Runtime,
	}
	return json.NewEncoder(os.Stdout).Encode(out)
}
