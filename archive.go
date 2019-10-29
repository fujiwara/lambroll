package lambroll

import (
	"archive/zip"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/pkg/errors"
)

// Archive archives zip
func (app *App) Archive(opt DeployOption) error {
	if err := (&opt).Expand(); err != nil {
		return errors.Wrap(err, "failed to validate deploy options")
	}

	zipfile, _, err := CreateZipArchive(*opt.SrcDir, opt.Excludes)
	if err != nil {
		return err
	}
	defer zipfile.Close()
	_, err = io.Copy(os.Stdout, zipfile)
	return err
}

// CreateZipArchive creates a zip archive
func CreateZipArchive(src string, excludes []string) (*os.File, os.FileInfo, error) {
	log.Printf("[info] creating zip archive from %s", src)
	tmpfile, err := ioutil.TempFile("", "archive")
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to open tempFile")
	}
	w := zip.NewWriter(tmpfile)
	err = filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
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
		return addToZip(w, path, relpath, info)
	})
	if err := w.Close(); err != nil {
		return nil, nil, errors.Wrap(err, "failed to create zip archive")
	}
	tmpfile.Seek(0, os.SEEK_SET)
	stat, _ := tmpfile.Stat()
	log.Printf("[info] zip archive wrote %d bytes", stat.Size())
	return tmpfile, stat, err
}

func matchExcludes(path string, excludes []string) bool {
	for _, pattern := range excludes {
		for _, name := range []string{path, filepath.Base(path)} {
			m, err := filepath.Match(pattern, name)
			if err != nil {
				log.Printf("[warn] failed to match exclude pattern %s to %s", pattern, name)
			}
			if m {
				return true
			}
		}
	}
	return false
}

func addToZip(z *zip.Writer, path, relpath string, info os.FileInfo) error {
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
	r, err := os.Open(path)
	if err != nil {
		log.Printf("[error] failed to open %s: %s", path, err)
		return err
	}
	defer r.Close()
	_, err = io.Copy(w, r)
	log.Printf("[debug] %s %10d %s %s",
		header.Mode(),
		header.UncompressedSize64,
		header.Modified.Format(time.RFC3339),
		header.Name,
	)
	return err
}

func (app *App) uploadFunctionToS3(f *os.File, bucket, key string) (string, error) {
	svc := s3.New(app.sess)
	log.Printf("[debug] PutObjcet to s3://%s/%s", bucket, key)
	// TODO multipart upload
	res, err := svc.PutObject(&s3.PutObjectInput{
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
