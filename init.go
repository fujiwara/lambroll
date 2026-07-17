package lambroll

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

// InitOption represents options for Init()
type InitOption struct {
	FunctionName   *string `help:"Function name for init" required:"true" default:"" json:"function_name,omitempty"`
	DownloadZip    bool    `name:"download" help:"Download function.zip" default:"false" json:"download,omitempty"`
	Unzip          bool    `help:"Unzip function.zip and delete it" default:"false" json:"unzip,omitempty"`
	Src            string  `help:"Source directory for unzipping function.zip" default:"." json:"src,omitempty"`
	Jsonnet        bool    `help:"render function.json as jsonnet" default:"false" json:"jsonnet,omitempty"`
	Qualifier      *string `help:"function version or alias" json:"qualifier,omitempty"`
	FunctionURL    bool    `help:"create function url definition file" default:"false" json:"function_url,omitempty"`
	ForceOverwrite bool    `help:"Overwrite existing files without prompting" default:"false" json:"force_overwrite,omitempty"`
}

// Init initializes function.json
func (app *App) Init(ctx context.Context, opt *InitOption) error {
	res, err := app.lambda.GetFunction(ctx, &lambda.GetFunctionInput{
		FunctionName: opt.FunctionName,
		Qualifier:    opt.Qualifier,
	})
	var c *types.FunctionConfiguration
	exists := true
	if err != nil {
		var nfe *types.ResourceNotFoundException
		if errors.As(err, &nfe) {
			slog.Info("function not found", "function", *opt.FunctionName)
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
		slog.Info("function found", "function", *opt.FunctionName)
		c = res.Configuration
	}

	var tags Tags
	if exists {
		arn := app.functionArn(ctx, *c.FunctionName)
		slog.Debug("listing tags", "arn", arn)
		res, err := app.lambda.ListTags(ctx, &lambda.ListTagsInput{
			Resource: aws.String(arn), // tags are not supported for alias
		})
		if err != nil {
			return fmt.Errorf("failed to list tags: %w", err)
		}
		tags = res.Tags
	}

	var code *types.FunctionCodeLocation
	if res != nil {
		code = res.Code
	}
	fn := newFunctionFrom(c, code, tags)
	if fn.Code != nil {
		// The version resolved at GetFunction becomes stale immediately after
		// the next upload. Writing it to the generated file would pin
		// deployments with --skip-archive to the old object.
		fn.Code.S3ObjectVersion = nil
	}

	if (opt.DownloadZip || opt.Unzip) && res != nil && res.Code != nil && aws.ToString(res.Code.RepositoryType) == "S3" {
		slog.Info("downloading file", "file", FunctionZipFilename)
		if err := download(ctx, *res.Code.Location, FunctionZipFilename); err != nil {
			return err
		}
		if opt.Unzip {
			if err := unzipAfterInit(ctx, FunctionZipFilename, opt.Src, opt.ForceOverwrite); err != nil {
				return err
			}
		}
	}

	slog.Info("creating file", "name", IgnoreFilename)
	err = app.saveFile(
		ctx,
		IgnoreFilename,
		[]byte(strings.Join(DefaultExcludes, "\n")+"\n"),
		os.FileMode(0644),
		opt.ForceOverwrite,
	)
	if err != nil {
		return err
	}

	var name string
	if opt.Jsonnet {
		name = DefaultFunctionFilenames[1]
	} else {
		name = DefaultFunctionFilenames[0]
	}
	slog.Info("creating file", "name", name)
	b, _ := marshalJSON(fn)
	if opt.Jsonnet {
		b, err = jsonToJsonnet(b, name)
		if err != nil {
			return err
		}
	}
	if err := app.saveFile(ctx, name, b, os.FileMode(0644), opt.ForceOverwrite); err != nil {
		return err
	}

	if opt.FunctionURL {
		if err := app.initFunctionURL(ctx, fn, exists, opt); err != nil {
			return err
		}
	}

	return nil
}

func download(ctx context.Context, url, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to new request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to get %s: %w", url, err)
	}
	defer resp.Body.Close()
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, os.FileMode(0644))
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", path, err)
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("failed to write file %s: %w", path, err)
	}
	return f.Close()
}

func unzipAfterInit(ctx context.Context, path, dest string, force bool) error {
	slog.Info("unzipping file", "file", path, "dest", dest)
	if err := unzip(ctx, path, dest, force); err != nil {
		return fmt.Errorf("failed to unzip %s: %w", path, err)
	}
	slog.Info("removing file", "file", path)
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to remove %s: %w", path, err)
	}
	return nil
}

func unzip(ctx context.Context, src, dest string, force bool) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	if err := os.MkdirAll(dest, 0755); err != nil {
		return err
	}

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)
		fi := f.FileInfo()
		if fi.IsDir() {
			slog.Debug("creating directory", "path", fpath)
			if err := os.MkdirAll(fpath, f.Mode()); err != nil {
				return err
			}
			continue
		}

		slog.Debug("extracting file", "path", fpath)
		if err := os.MkdirAll(filepath.Dir(fpath), 0755); err != nil {
			return err
		}

		fc, err := f.Open()
		if err != nil {
			return err
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			// supports for symbolic link
			if err := saveSymlinkIO(ctx, fpath, fc); err != nil {
				return err
			}
		} else {
			// normal file
			if err := saveFileIO(ctx, fpath, fc, f.Mode(), force); err != nil {
				return err
			}
		}
	}

	return nil
}

func saveSymlinkIO(_ context.Context, fpath string, r io.ReadCloser) error {
	defer r.Close()
	l, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	linkTo := string(l)
	slog.Debug("writing symlink", "path", fpath, "target", linkTo)

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	defer os.Chdir(cwd)
	if err := os.Chdir(filepath.Dir(fpath)); err != nil {
		return err
	}
	name := filepath.Base(fpath)
	slog.Debug("creating symlink", "name", name, "target", linkTo)
	return os.Symlink(linkTo, name)
}
