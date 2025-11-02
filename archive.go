package lambroll

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/fujiwara/lambroll/wildcard"
)

type ArchiveOption struct {
	Src  string `help:"function zip archive or src dir" default:"."`
	Dest string `help:"destination file path" default:"function.zip"`

	ZipOption
}

// Archive archives zip
func (app *App) Archive(ctx context.Context, opt *ArchiveOption) error {
	if err := opt.Expand(); err != nil {
		return err
	}

	zipfile, _, err := createZipArchive(opt.Src, opt.excludes, opt.KeepSymlink)
	if err != nil {
		return err
	}
	defer zipfile.Close()
	var w io.WriteCloser
	if opt.Dest == "-" {
		slog.Info("writing zip archive to stdout")
		w = os.Stdout
	} else {
		slog.Info("writing zip archive", "dest", opt.Dest)
		w, err = os.Create(opt.Dest)
		if err != nil {
			return fmt.Errorf("failed to create %s: %w", opt.Dest, err)
		}
		defer w.Close()
	}
	_, err = io.Copy(w, zipfile)
	return err
}

func loadZipArchive(src string) (*os.File, os.FileInfo, error) {
	slog.Info("reading zip archive", "src", src)
	r, err := zip.OpenReader(src)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open zip file %s: %w", src, err)
	}
	for _, f := range r.File {
		header := f.FileHeader
		slog.Debug("zip entry",
			"mode", header.Mode(),
			"size", header.UncompressedSize64,
			"modified", header.Modified.Format(time.RFC3339),
			"name", header.Name,
		)
	}
	r.Close()
	info, err := os.Stat(src)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to stat %s: %w", src, err)
	}
	slog.Info("zip archive", "bytes", info.Size())
	fh, err := os.Open(src)
	return fh, info, err
}

// createZipArchive creates a zip archive
func createZipArchive(src string, excludes []string, keepSymlink bool) (*os.File, os.FileInfo, error) {
	slog.Info("creating zip archive", "src", src)
	tmpfile, err := os.CreateTemp("", "archive")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open tempFile: %w", err)
	}
	w := zip.NewWriter(tmpfile)
	err = filepath.WalkDir(src, func(path string, info fs.DirEntry, err error) error {
		slog.Debug("walking", "path", path)
		if err != nil {
			slog.Error("failed to walking dir", "src", src)
			return err
		}
		if info.IsDir() {
			return nil
		}
		relpath, _ := filepath.Rel(src, path)
		if matchExcludes(relpath, excludes) {
			slog.Debug("skipping", "path", relpath)
			return nil
		}
		slog.Debug("adding", "path", relpath)
		return addToZip(w, path, relpath, info, keepSymlink)
	})
	if err := w.Close(); err != nil {
		return nil, nil, fmt.Errorf("failed to create zip archive: %w", err)
	}
	tmpfile.Seek(0, io.SeekStart)
	stat, _ := tmpfile.Stat()
	slog.Info("zip archive wrote", "bytes", stat.Size())
	return tmpfile, stat, err
}

func matchExcludes(path string, excludes []string) bool {
	for _, pattern := range excludes {
		if wildcard.Match(pattern, path) {
			return true
		}
	}
	return false
}

func followSymlink(path string) (string, fs.FileInfo, error) {
	link, err := os.Readlink(path)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read symlink %s: %s", path, err)
	}
	linkTarget := filepath.Join(filepath.Dir(path), link)
	slog.Debug("resolve symlink", "path", path, "target", linkTarget)
	info, err := os.Stat(linkTarget)
	if err != nil {
		return "", nil, fmt.Errorf("failed to stat symlink target %s: %s", linkTarget, err)
	}
	if info.IsDir() {
		return "", nil, fmt.Errorf("symlink target is a directory %s", linkTarget)
	}
	return linkTarget, info, nil
}

func addToZip(z *zip.Writer, path, relpath string, entry fs.DirEntry, keepSymlink bool) error {
	info, err := entry.Info()
	if err != nil {
		slog.Error("failed to get info", "path", path, "error", err)
		return err
	}
	var reader io.ReadCloser
	if info.Mode()&fs.ModeSymlink != 0 { // is symlink
		if keepSymlink {
			link, err := os.Readlink(path)
			if err != nil {
				return fmt.Errorf("failed to read symlink %s: %w", path, err)
			}
			reader = io.NopCloser(strings.NewReader(link))
		} else {
			// treat symlink as file. skip symlink target directory.
			path, info, err = followSymlink(path) // overwrite path, info
			if err != nil {
				slog.Warn("failed to follow symlink. skip", "error", err)
				return nil
			}
		}
	}
	if reader == nil {
		reader, err = os.Open(path)
		if err != nil {
			slog.Error("failed to open", "path", path, "error", err)
			return err
		}
	}
	defer reader.Close()

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		slog.Error("failed to create zip file header", "error", err)
		return err
	}
	header.Name = relpath // fix name as subdir
	header.Method = zip.Deflate
	w, err := z.CreateHeader(header)
	if err != nil {
		slog.Error("failed to create in zip", "error", err)
		return err
	}
	_, err = io.Copy(w, reader)
	slog.Debug("zip entry",
		"mode", header.Mode(),
		"size", header.UncompressedSize64,
		"modified", header.Modified.Format(time.RFC3339),
		"name", header.Name,
	)
	return err
}

func (app *App) uploadFunctionToS3(ctx context.Context, f *os.File, bucket, key string) (string, error) {
	svc := s3.NewFromConfig(app.awsConfig)
	slog.Debug("PutObject to S3", "bucket", bucket, "key", key)
	res, err := svc.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   f,
	})
	if err != nil {
		return "", err
	}
	if res.VersionId != nil {
		return *res.VersionId, nil
	}
	return "", nil // not versioned
}
