package lambroll

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"log"
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

	ExcludeFileOption
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
		log.Printf("[info] writing zip archive to stdout")
		w = os.Stdout
	} else {
		log.Printf("[info] writing zip archive to %s", opt.Dest)
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
	log.Printf("[info] reading zip archive from %s", src)
	r, err := zip.OpenReader(src)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open zip file %s: %w", src, err)
	}
	for _, f := range r.File {
		header := f.FileHeader
		log.Printf("[debug] %s %10d %s %s",
			header.Mode(),
			header.UncompressedSize64,
			header.Modified.Format(time.RFC3339),
			header.Name,
		)
	}
	r.Close()
	info, err := os.Stat(src)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to stat %s: %w", src, err)
	}
	log.Printf("[info] zip archive %d bytes", info.Size())
	fh, err := os.Open(src)
	return fh, info, err
}

// createZipArchive creates a zip archive
func createZipArchive(src string, excludes []string, keepSymlink bool) (*os.File, os.FileInfo, error) {
	log.Printf("[info] creating zip archive from %s", src)
	tmpfile, err := os.CreateTemp("", "archive")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open tempFile: %w", err)
	}
	w := zip.NewWriter(tmpfile)
	err = filepath.WalkDir(src, func(path string, info fs.DirEntry, err error) error {
		log.Println("[trace] waking", path)
		if err != nil {
			log.Println("[error] failed to walking dir in", src)
			return err
		}
		if info.IsDir() {
			return nil
		}
		relpath, _ := filepath.Rel(src, path)
		if matchExcludes(relpath, excludes) {
			log.Println("[trace] skipping", relpath)
			return nil
		}
		log.Println("[trace] adding", relpath)
		return addToZip(w, path, relpath, info, keepSymlink)
	})
	if err := w.Close(); err != nil {
		return nil, nil, fmt.Errorf("failed to create zip archive: %w", err)
	}
	tmpfile.Seek(0, io.SeekStart)
	stat, _ := tmpfile.Stat()
	log.Printf("[info] zip archive wrote %d bytes", stat.Size())
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
	log.Printf("[debug] resolve symlink %s to %s", path, linkTarget)
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
		log.Printf("[error] failed to get info %s: %s", path, err)
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
				log.Printf("[warn] failed to follow symlink. skip: %s", err)
				return nil
			}
		}
	}
	if reader == nil {
		reader, err = os.Open(path)
		if err != nil {
			log.Printf("[error] failed to open %s: %s", path, err)
			return err
		}
	}
	defer reader.Close()

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		log.Println("[error] failed to create zip file header", err)
		return err
	}
	header.Name = relpath // fix name as subdir
	header.Method = zip.Deflate
	w, err := z.CreateHeader(header)
	if err != nil {
		log.Println("[error] failed to create in zip", err)
		return err
	}
	_, err = io.Copy(w, reader)
	log.Printf("[debug] %s %10d %s %s",
		header.Mode(),
		header.UncompressedSize64,
		header.Modified.Format(time.RFC3339),
		header.Name,
	)
	return err
}

func (app *App) uploadFunctionToS3(ctx context.Context, f *os.File, bucket, key string) (string, error) {
	svc := s3.NewFromConfig(app.awsConfig)
	log.Printf("[debug] PutObject to s3://%s/%s", bucket, key)
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
