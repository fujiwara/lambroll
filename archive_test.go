package lambroll_test

import (
	"archive/zip"
	"os"
	"testing"
	"time"

	"github.com/fujiwara/lambroll"
)

func TestCreateZipArchive(t *testing.T) {
	r, err := lambroll.CreateZipArchive("test/src", []string{"*.bin", "skip/*"})
	if err != nil {
		t.Error("faile to CreateZipArchive", err)
	}
	defer r.Close()
	defer os.Remove(r.Name())

	zr, err := zip.OpenReader(r.Name())
	if err != nil {
		t.Error("failed to new zip reader", err)
	}
	if len(zr.File) != 3 {
		t.Errorf("unexpected included files num %d expect %d", len(zr.File), 3)
	}
	for _, f := range zr.File {
		h := f.FileHeader
		t.Logf("%s %s %s", h.Mode(), h.Modified.Format(time.RFC3339), h.Name)
	}
}
