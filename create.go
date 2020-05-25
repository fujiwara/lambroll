package lambroll

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/pkg/errors"
)

var directUploadThreshold = int64(50 * 1024 * 1024) // 50MB

func (app *App) prepareFunctionCodeForDeploy(opt DeployOption, fn *Function) error {
	var (
		zipfile *os.File
		info    os.FileInfo
	)

	src := *opt.Src
	if fi, err := os.Stat(src); err != nil {
		return errors.Wrapf(err, "src %s is not found", src)
	} else if fi.IsDir() {
		zipfile, info, err = createZipArchive(src, opt.Excludes)
		if err != nil {
			return err
		}
		defer os.Remove(zipfile.Name())
	} else if !fi.IsDir() {
		zipfile, info, err = loadZipArchive(src)
		if err != nil {
			return err
		}
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
	err := app.prepareFunctionCodeForDeploy(opt, fn)
	if err != nil {
		return errors.Wrap(err, "failed to prepare function code")
	}
	log.Println("[info] creating function", opt.label())
	log.Println("[debug]\n", fn.String())

	version := "(created)"
	if !*opt.DryRun {
		fn.Publish = opt.Publish
		res, err := app.lambda.CreateFunction(fn)
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
