package lambroll_test

import (
	"archive/zip"
	"testing"

	"github.com/fujiwara/lambroll"
)

func TestCreateZipArchive(t *testing.T) {
	r, err := lambroll.CreateZipArchive("test/src", []string{"*.bin", "skip/*"})
	if err != nil {
		t.Error("faile to CreateZipArchive", err)
	}
	defer r.Close()

	zr, err := zip.OpenReader(r.Name())
	if err != nil {
		t.Error("failed to new zip reader", err)
	}
	if len(zr.File) != 3 {
		t.Errorf("unexpected included files num %d expect %d", len(zr.File), 3)
	}
}
