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
			return nil
		}
		relpath, _ := filepath.Rel(src, path)
		if matchExcludes(relpath, excludes) {
			log.Println("[debug] skipping", relpath)
			return nil
		}
		log.Println("[debug] adding", relpath)
		return addToZip(w, path, relpath, info)
	})
	if err := w.Close(); err != nil {
		return nil, errors.Wrap(err, "failed to create zip archive")
	}
	tmpfile.Seek(0, os.SEEK_SET)
	return tmpfile, err
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
	return err
}
