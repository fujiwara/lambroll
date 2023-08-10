package lambroll

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/pkg/errors"
)

var directUploadThreshold = int64(50 * 1024 * 1024) // 50MB

func prepareZipfile(src string, excludes []string) (*os.File, os.FileInfo, error) {
	if fi, err := os.Stat(src); err != nil {
		return nil, nil, errors.Wrapf(err, "src %s is not found", src)
	} else if fi.IsDir() {
		zipfile, info, err := createZipArchive(src, excludes)
		if err != nil {
			return nil, nil, err
		}
		return zipfile, info, nil
	} else if !fi.IsDir() {
		zipfile, info, err := loadZipArchive(src)
		if err != nil {
			return nil, nil, err
		}
		return zipfile, info, nil
	}
	return nil, nil, fmt.Errorf("src %s is not found", src)
}

func (app *App) prepareFunctionCodeForDeploy(opt DeployOption, fn *Function) error {
	if aws.StringValue(fn.PackageType) == packageTypeImage {
		if fn.Code == nil || fn.Code.ImageUri == nil {
			return errors.New("PackageType=Image requires Code.ImageUri in function definition")
		}
		// deploy docker image. no need to preprare
		log.Printf("[info] using docker image %s", *fn.Code.ImageUri)

		if fn.ImageConfig == nil {
			fn.ImageConfig = &lambda.ImageConfig{} // reset explicitly
		}
		return nil
	}

	if opt.SkipArchive != nil && *opt.SkipArchive {
		if fn.Code == nil || fn.Code.S3Bucket == nil || fn.Code.S3Key == nil {
			return errors.New("--skip-archive requires Code.S3Bucket and Code.S3key elements in function definition")
		}
		return nil
	}

	zipfile, info, err := prepareZipfile(*opt.Src, opt.Excludes)
	if err != nil {
		return err
	}
	defer zipfile.Close()

	if fn.Code != nil {
		if bucket, key := fn.Code.S3Bucket, fn.Code.S3Key; bucket != nil && key != nil {
			log.Printf("[info] uploading function %d bytes to s3://%s/%s", info.Size(), *bucket, *key)
			versionID, err := app.uploadFunctionToS3(zipfile, *bucket, *key)
			if err != nil {
				errors.Wrapf(err, "failed to upload function zip to s3://%s/%s", *bucket, *key)
			}
			if versionID != "" {
				log.Printf("[info] object created as version %s", versionID)
				fn.Code.S3ObjectVersion = aws.String(versionID)
			} else {
				log.Printf("[info] object created")
				fn.Code.S3ObjectVersion = nil
			}
		} else {
			return errors.New("Code.S3Bucket or Code.S3Key are not defined")
		}
	} else {
		// try direct upload
		if s := info.Size(); s > directUploadThreshold {
			return fmt.Errorf("cannot use a zip file for update function directly. Too large file %d bytes. Please define Code.S3Bucket and Code.S3Key in function.json", s)
		}
		b, err := ioutil.ReadAll(zipfile)
		if err != nil {
			return errors.Wrap(err, "failed to read zipfile content")
		}
		fn.Code = &lambda.FunctionCode{ZipFile: b}
	}
	return nil
}

func (app *App) create(opt DeployOption, fn *Function) error {
	ctx := context.Background()
	err := app.prepareFunctionCodeForDeploy(opt, fn)
	if err != nil {
		return errors.Wrap(err, "failed to prepare function code")
	}
	log.Println("[info] creating function", opt.label())
	log.Println("[debug]\n", fn.String())

	version := "(created)"
	if !*opt.DryRun {
		fn.Publish = opt.Publish
		res, err := app.createFunction(ctx, fn)
		if err != nil {
			return errors.Wrap(err, "failed to create function")
		}
		if res.Version != nil {
			version = *res.Version
			log.Printf("[info] deployed function version %s", version)
		} else {
			log.Println("[info] deployed")
		}
	}

	if err := app.updateTags(fn, opt); err != nil {
		return err
	}

	if !*opt.Publish {
		return nil
	}

	log.Printf("[info] creating alias set %s to version %s %s", *opt.AliasName, version, opt.label())
	if !*opt.DryRun {
		alias, err := app.lambda.CreateAlias(&lambda.CreateAliasInput{
			FunctionName:    fn.FunctionName,
			FunctionVersion: aws.String(version),
			Name:            aws.String(*opt.AliasName),
		})
		if err != nil {
			return errors.Wrap(err, "failed to create alias")
		}
		log.Println("[info] alias created")
		log.Printf("[debug]\n%s", alias.String())
	}
	return nil
}

func (app *App) createFunction(ctx context.Context, fn *lambda.CreateFunctionInput) (*lambda.FunctionConfiguration, error) {
	if res, err := app.lambda.CreateFunctionWithContext(ctx, fn); err != nil {
		return nil, errors.Wrap(err, "failed to create function")
	} else {
		return res, app.waitForLastUpdateStatusSuccessful(ctx, *fn.FunctionName)
	}
}
