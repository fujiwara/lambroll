package lambroll

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

var directUploadThreshold = int64(50 * 1024 * 1024) // 50MB

func prepareZipfile(src string, excludes []string, keepSymlink bool) (*os.File, os.FileInfo, error) {
	if fi, err := os.Stat(src); err != nil {
		return nil, nil, fmt.Errorf("src %s is not found: %w", src, err)
	} else if fi.IsDir() {
		zipfile, info, err := createZipArchive(src, excludes, keepSymlink)
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

func (app *App) prepareFunctionCodeForDeploy(ctx context.Context, opt *DeployOption, fn *Function) error {
	if fn.PackageType == types.PackageTypeImage {
		if fn.Code == nil || fn.Code.ImageUri == nil {
			return fmt.Errorf("PackageType=Image requires Code.ImageUri in function definition")
		}
		// deploy docker image. no need to prepare
		log.Printf("[info] using docker image %s", *fn.Code.ImageUri)

		if fn.ImageConfig == nil {
			fn.ImageConfig = &types.ImageConfig{} // reset explicitly
		}
		return nil
	}

	if opt.SkipArchive {
		if fn.Code == nil || fn.Code.S3Bucket == nil || fn.Code.S3Key == nil {
			return fmt.Errorf("--skip-archive requires Code.S3Bucket and Code.S3key elements in function definition")
		}
		return nil
	}

	zipfile, info, err := prepareZipfile(opt.Src, opt.excludes, opt.KeepSymlink)
	if err != nil {
		return err
	}
	defer zipfile.Close()

	if fn.Code != nil {
		if bucket, key := fn.Code.S3Bucket, fn.Code.S3Key; bucket != nil && key != nil {
			log.Printf("[info] uploading function %d bytes to s3://%s/%s", info.Size(), *bucket, *key)
			versionID, err := app.uploadFunctionToS3(ctx, zipfile, *bucket, *key)
			if err != nil {
				return fmt.Errorf("failed to upload function zip to s3://%s/%s: %w", *bucket, *key, err)
			}
			if versionID != "" {
				log.Printf("[info] object created as version %s", versionID)
				fn.Code.S3ObjectVersion = aws.String(versionID)
			} else {
				log.Printf("[info] object created")
				fn.Code.S3ObjectVersion = nil
			}
		} else {
			return fmt.Errorf("Code.S3Bucket or Code.S3Key are not defined")
		}
	} else {
		// try direct upload
		if s := info.Size(); s > directUploadThreshold {
			return fmt.Errorf("cannot use a zip file for update function directly. Too large file %d bytes. Please define Code.S3Bucket and Code.S3Key in function.json", s)
		}
		b, err := io.ReadAll(zipfile)
		if err != nil {
			return fmt.Errorf("failed to read zipfile content: %w", err)
		}
		fn.Code = &types.FunctionCode{ZipFile: b}
	}
	return nil
}

func (app *App) create(ctx context.Context, opt *DeployOption, fn *Function) error {
	err := app.prepareFunctionCodeForDeploy(ctx, opt, fn)
	if err != nil {
		return fmt.Errorf("failed to prepare function code: %w", err)
	}
	log.Println("[info] creating function", opt.label())

	version := "(created)"
	if !opt.DryRun {
		fn.Publish = opt.Publish
		res, err := app.createFunction(ctx, fn)
		if err != nil {
			return fmt.Errorf("failed to create function: %w", err)
		}
		if res.Version != nil {
			version = *res.Version
			log.Printf("[info] deployed function version %s", version)
		} else {
			log.Println("[info] deployed")
		}
	}

	if err := app.updateTags(ctx, fn, opt); err != nil {
		return err
	}

	if !opt.Publish {
		return nil
	}

	log.Printf("[info] creating alias set %s to version %s %s", opt.AliasName, version, opt.label())
	if !opt.DryRun {
		_, err := app.lambda.CreateAlias(ctx, &lambda.CreateAliasInput{
			FunctionName:    fn.FunctionName,
			FunctionVersion: aws.String(version),
			Name:            aws.String(opt.AliasName),
		})
		if err != nil {
			return fmt.Errorf("failed to create alias: %w", err)
		}
		log.Println("[info] alias created")
	}
	return nil
}

func (app *App) createFunction(ctx context.Context, fn *Function) (*lambda.CreateFunctionOutput, error) {
	in := lambda.CreateFunctionInput(*fn)
	if res, err := app.lambda.CreateFunction(ctx, &in); err != nil {
		return nil, fmt.Errorf("failed to create function: %w", err)
	} else {
		return res, app.waitForLastUpdateStatusSuccessful(ctx, *fn.FunctionName)
	}
}
