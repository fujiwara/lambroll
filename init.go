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

	awsv2 "github.com/aws/aws-sdk-go-v2/aws"
	lambdav2 "github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdav2types "github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

// InitOption represents options for Init()
type InitOption struct {
	FunctionName *string
	DownloadZip  *bool
}

// Init initializes function.json
func (app *App) Init(opt InitOption) error {
	ctx := context.TODO()
	res, err := app.lambdav2.GetFunction(ctx, &lambdav2.GetFunctionInput{
		FunctionName: opt.FunctionName,
	})
	var c *lambdav2types.FunctionConfiguration
	exists := true
	if err != nil {
		var nfe *lambdav2types.ResourceNotFoundException
		if errors.As(err, &nfe) {
			log.Printf("[info] function %s is not found", *opt.FunctionName)
			c = &lambdav2types.FunctionConfiguration{
				FunctionName: opt.FunctionName,
				MemorySize:   awsv2.Int32(128),
				Runtime:      lambdav2types.RuntimeNodejs18x,
				Timeout:      awsv2.Int32(3),
				Handler:      awsv2.String("index.handler"),
				Role: awsv2.String(
					fmt.Sprintf(
						"arn:aws:iam::%s:role/YOUR_LAMBDA_ROLE_NAME",
						app.AWSAccountID(),
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

	var tags TagsV2
	if exists {
		arn := app.functionArn(*c.FunctionName)
		log.Printf("[debug] listing tags of %s", arn)
		res, err := app.lambdav2.ListTags(ctx, &lambdav2.ListTagsInput{
			Resource: awsv2.String(arn),
		})
		if err != nil {
			return fmt.Errorf("faled to list tags: %w", err)
		}
		tags = res.Tags
	}

	fn := newFunctionFromV2(c, res.Code, tags)

	if *opt.DownloadZip && res.Code != nil && *res.Code.RepositoryType == "S3" {
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

	log.Printf("[info] creating %s", FunctionFilenames[0])
	b, _ := marshalJSONV2(fn)
	return app.saveFile(FunctionFilenames[0], b, os.FileMode(0644))
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
