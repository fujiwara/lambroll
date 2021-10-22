package lambroll

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/pkg/errors"
)

// InitOption represents options for Init()
type InitOption struct {
	FunctionName *string
	DownloadZip  *bool
}

// Init initializes function.json
func (app *App) Init(opt InitOption) error {
	res, err := app.lambda.GetFunction(&lambda.GetFunctionInput{
		FunctionName: opt.FunctionName,
	})
	var c *lambda.FunctionConfiguration
	exists := true
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case lambda.ErrCodeResourceNotFoundException:
				log.Printf("[info] function %s is not found", *opt.FunctionName)
				c = &lambda.FunctionConfiguration{
					FunctionName: opt.FunctionName,
					MemorySize:   aws.Int64(128),
					Runtime:      aws.String("nodejs10.x"),
					Timeout:      aws.Int64(3),
					Handler:      aws.String("index.handler"),
					Role: aws.String(
						fmt.Sprintf(
							"arn:aws:iam::%s:role/YOUR_LAMBDA_ROLE_NAME",
							app.AWSAccountID(),
						),
					),
				}
				exists = false
			default:
			}
		}
		if c == nil {
			return errors.Wrap(err, "failed to GetFunction"+*opt.FunctionName)
		}
	} else {
		log.Printf("[info] function %s found", *opt.FunctionName)
		c = res.Configuration
	}

	var tags Tags
	if exists {
		arn := app.functionArn(*c.FunctionName)
		log.Printf("[debug] listing tags of %s", arn)
		res, err := app.lambda.ListTags(&lambda.ListTagsInput{
			Resource: aws.String(arn),
		})
		if err != nil {
			return errors.Wrap(err, "faled to list tags")
		}
		tags = res.Tags
	}

	fn := newFuctionFrom(c, tags)

	if *opt.DownloadZip && res.Code != nil && *res.Code.RepositoryType == "S3" {
		log.Printf("[info] downloading %s", FunctionZipFilename)
		if err := download(*res.Code.Location, FunctionZipFilename); err != nil {
			return err
		}
	}
	if aws.StringValue(res.Configuration.PackageType) == "Image" {
		log.Printf("[debug] Image URL=%s", *res.Code.ImageUri)
		fn.PackageType = aws.String("Image")
		fn.Code = &lambda.FunctionCode{
			ImageUri: res.Code.ImageUri,
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

	log.Printf("[info] creating %s", FunctionFilename)
	b, _ := marshalJSON(fn)
	return app.saveFile(FunctionFilename, b, os.FileMode(0644))
}

func download(url, path string) error {
	resp, err := http.Get(url)
	if err != nil {
		return errors.Wrapf(err, "failed to get %s", url)
	}
	defer resp.Body.Close()
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, os.FileMode(0644))
	if err != nil {
		return errors.Wrapf(err, "failed to open file %s", path)
	}
	_, err = io.Copy(f, resp.Body)
	return err
}
