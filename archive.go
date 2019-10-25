package lambroll

import (
	"archive/zip"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

// CreateZipArchive creates a zip archive
func CreateZipArchive(src string, excludes []string) (*os.File, error) {
	tmpfile, err := ioutil.TempFile("", "archive")
	if err != nil {
		return nil, errors.Wrap(err, "failed to open tempFile")
	}
	w := zip.NewWriter(tmpfile)
	err = filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		log.Println("[debug] waking", path)
		if err != nil {
			log.Println("[error] failed to walking dir in", src)
			return err
		}
		if info.IsDir() {
			log.Println("[debug] skipping dir", path)
			return nil
		}
		relpath, _ := filepath.Rel(src, path)
		for _, pattern := range excludes {
			log.Printf("[debug] match pattern %s to %s", pattern, path)
			for _, name := range []string{relpath, filepath.Base(path)} {
				m, err := filepath.Match(pattern, name)
				if err != nil {
					log.Printf("[warn] failed to match exclude pattern %s to %s", pattern, name)
				}
				if m {
					log.Printf("[debug] skip matched exclude pattern %s to %s", pattern, name)
					return nil
				}
			}
		}
		f, err := w.Create(relpath)
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
		_, err = io.Copy(f, r)
		return err
	})
	if err := w.Close(); err != nil {
		return nil, errors.Wrap(err, "failed to create zip archive")
	}
	tmpfile.Seek(0, os.SEEK_SET)
	return tmpfile, err
}
