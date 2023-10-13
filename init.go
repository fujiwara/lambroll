package lambroll

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

// InitOption represents options for Init()
type InitOption struct {
	FunctionName *string `help:"Function name for init" required:"true" default:""`
	DownloadZip  bool    `help:"Download function.zip" default:"false"`
}

// Init initializes function.json
func (app *App) Init(ctx context.Context, opt *InitOption) error {
	res, err := app.lambda.GetFunction(ctx, &lambda.GetFunctionInput{
		FunctionName: opt.FunctionName,
	})
	var c *types.FunctionConfiguration
	exists := true
	if err != nil {
		var nfe *types.ResourceNotFoundException
		if errors.As(err, &nfe) {
			log.Printf("[info] function %s is not found", *opt.FunctionName)
			c = &types.FunctionConfiguration{
				FunctionName: opt.FunctionName,
				MemorySize:   aws.Int32(128),
				Runtime:      types.RuntimeNodejs18x,
				Timeout:      aws.Int32(3),
				Handler:      aws.String("index.handler"),
				Role: aws.String(
					fmt.Sprintf(
						"arn:aws:iam::%s:role/YOUR_LAMBDA_ROLE_NAME",
						app.AWSAccountID(ctx),
					),
				),
			}
			exists = false
		}
		if c == nil {
			return fmt.Errorf("failed to GetFunction %s: %w", *opt.FunctionName, err)
		}
	} else {
		log.Printf("[info] function %s found", *opt.FunctionName)
		c = res.Configuration
	}

	var tags Tags
	if exists {
		arn := app.functionArn(ctx, *c.FunctionName)
		log.Printf("[debug] listing tags of %s", arn)
		res, err := app.lambda.ListTags(ctx, &lambda.ListTagsInput{
			Resource: aws.String(arn),
		})
		if err != nil {
			return fmt.Errorf("faled to list tags: %w", err)
		}
		tags = res.Tags
	}

	fn := newFunctionFrom(c, res.Code, tags)

	if opt.DownloadZip && res.Code != nil && *res.Code.RepositoryType == "S3" {
		log.Printf("[info] downloading %s", FunctionZipFilename)
		if err := download(*res.Code.Location, FunctionZipFilename); err != nil {
			return err
		}
	}

	log.Printf("[info] creating %s", IgnoreFilename)
	err = app.saveFile(
		IgnoreFilename,
		[]byte(strings.Join(DefaultExcludes, "\n")+"\n"),
		os.FileMode(0644),
	)
	if err != nil {
		return err
	}

	log.Printf("[info] creating %s", DefaultFunctionFilenames[0])
	b, _ := marshalJSON(fn)
	return app.saveFile(DefaultFunctionFilenames[0], b, os.FileMode(0644))
}

func download(url, path string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to get %s: %w", url, err)
	}
	defer resp.Body.Close()
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, os.FileMode(0644))
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", path, err)
	}
	_, err = io.Copy(f, resp.Body)
	return err
}
