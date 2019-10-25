package lambroll

import (
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/pkg/errors"
)

type InitOption struct {
	FunctionName *string
}

func (app *App) Init(opt InitOption) error {
	res, err := app.lambda.GetFunction(&lambda.GetFunctionInput{
		FunctionName: opt.FunctionName,
	})
	var c *lambda.FunctionConfiguration
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case lambda.ErrCodeResourceNotFoundException:
				log.Printf("[info] function %s is not found", *opt.FunctionName)
				c = &lambda.FunctionConfiguration{
					FunctionName: opt.FunctionName,
					MemorySize:   aws.Int64(128),
					Runtime:      aws.String(""),
					Timeout:      aws.Int64(5),
					Handler:      aws.String(""),
					Role: aws.String(
						fmt.Sprintf("arn:aws:iam:%s:role/YOUR_LAMBDA_ROLE_NAME", app.accountID),
					),
				}
			default:
			}
		}
		if c == nil {
			return errors.Wrap(err, "failed to GetFunciton"+*opt.FunctionName)
		}
	} else {
		log.Printf("[info] function %s found", *opt.FunctionName)
		c = res.Configuration
	}
	fn := &lambda.CreateFunctionInput{
		Description:  c.Description,
		FunctionName: c.FunctionName,
		Handler:      c.Handler,
		MemorySize:   c.MemorySize,
		Role:         c.Role,
		Runtime:      c.Runtime,
		Timeout:      c.Timeout,
	}
	if e := c.Environment; e != nil {
		fn.Environment = &lambda.Environment{
			Variables: e.Variables,
		}
	}
	for _, layer := range c.Layers {
		fn.Layers = append(fn.Layers, layer.Arn)
	}
	if t := c.TracingConfig; t != nil {
		fn.TracingConfig = &lambda.TracingConfig{
			Mode: t.Mode,
		}
	}
	if v := c.VpcConfig; v != nil && *v.VpcId != "" {
		fn.VpcConfig = &lambda.VpcConfig{
			SubnetIds:        v.SubnetIds,
			SecurityGroupIds: v.SecurityGroupIds,
		}
	}

	b, _ := marshalJSON(fn)
	return app.saveFile("function.json", b, os.FileMode(0644))
}
