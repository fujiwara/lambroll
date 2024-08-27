package lambroll_test

import (
	"archive/zip"
	"fmt"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/fujiwara/lambroll"
	"github.com/google/go-cmp/cmp"
)

type zipTestSuite struct {
	WorkingDir  string
	SrcDir      string
	Expected    []string
	KeepSymlink bool
}

func (s zipTestSuite) String() string {
	return fmt.Sprintf("%s_src_%s", s.WorkingDir, s.SrcDir)
}

var createZipArchives = []zipTestSuite{
	{
		WorkingDir:  ".",
		SrcDir:      "test/src",
		Expected:    []string{"dir/sub.txt", "ext-hello.txt", "hello.symlink", "hello.txt", "index.js", "world"},
		KeepSymlink: false,
	},
	{
		WorkingDir:  "test/src/dir",
		SrcDir:      "../",
		Expected:    []string{"dir/sub.txt", "ext-hello.txt", "hello.symlink", "hello.txt", "index.js", "world"},
		KeepSymlink: false,
	},
	{
		WorkingDir:  ".",
		SrcDir:      "test/src",
		Expected:    []string{"dir/sub.txt", "dir.symlink", "ext-hello.txt", "hello.symlink", "hello.txt", "index.js", "world"},
		KeepSymlink: true,
	},
}

func TestCreateZipArchive(t *testing.T) {
	for _, s := range createZipArchives {
		t.Run(s.String(), func(t *testing.T) {
			testCreateZipArchive(t, s)
		})
	}
}

func testCreateZipArchive(t *testing.T, s zipTestSuite) {
	cwd, _ := os.Getwd()
	os.Chdir(s.WorkingDir)
	defer os.Chdir(cwd)

	excludes := []string{}
	excludes = append(excludes, lambroll.DefaultExcludes...)
	excludes = append(excludes, []string{"*.bin", "skip/*"}...)
	r, info, err := lambroll.CreateZipArchive(s.SrcDir, excludes, s.KeepSymlink)
	if err != nil {
		t.Error("failed to CreateZipArchive", err)
	}
	defer r.Close()
	defer os.Remove(r.Name())

	zr, err := zip.OpenReader(r.Name())
	if err != nil {
		t.Error("failed to new zip reader", err)
	}
	if len(zr.File) != len(s.Expected) {
		t.Errorf("unexpected included files num %d expect %d", len(zr.File), len(s.Expected))
	}
	zipFiles := []string{}
	for _, f := range zr.File {
		h := f.FileHeader
		t.Logf("%s %10d %s %s",
			h.Mode(),
			h.UncompressedSize64,
			h.Modified.Format(time.RFC3339),
			h.Name,
		)
		zipFiles = append(zipFiles, h.Name)
	}
	slices.Sort(zipFiles)
	slices.Sort(s.Expected)
	if diff := cmp.Diff(zipFiles, s.Expected); diff != "" {
		t.Errorf("unexpected included files %s", diff)
	}

	if info.Size() < 100 {
		t.Errorf("too small file got %d bytes", info.Size())
	}
}

func TestLoadZipArchive(t *testing.T) {
	r, info, err := lambroll.LoadZipArchive("test/src.zip")
	if err != nil {
		t.Error("failed to LoadZipArchive", err)
	}
	defer r.Close()

	if info.Size() < 100 {
		t.Errorf("too small file got %d bytes", info.Size())
	}
}

func TestLoadNotZipArchive(t *testing.T) {
	_, _, err := lambroll.LoadZipArchive("test/src/hello.txt")
	if err == nil {
		t.Error("must be failed to load not a zip file")
	}
	t.Log(err)
}
