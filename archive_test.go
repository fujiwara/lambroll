package lambroll_test

import (
	"archive/zip"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
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

func TestUnzip(t *testing.T) {
	ctx := context.TODO()
	dest := t.TempDir()
	if err := lambroll.Unzip(ctx, "test/src.zip", dest, false); err != nil {
		t.Error("failed to Unzip", err)
	}
	unzipEntries := []string{}
	err := filepath.WalkDir(dest, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dest, path)
		unzipEntries = append(unzipEntries, rel)
		return nil
	})
	if err != nil {
		t.Error("failed to walk", err)
	}
	sort.Strings(unzipEntries)
	expected := []string{"dir.symlink", "dir/sub.txt", "hello.symlink", "hello.txt", "world"}
	if diff := cmp.Diff(unzipEntries, expected); diff != "" {
		t.Errorf("unexpected unzip entries %s", diff)
	}

	// debug
	o, _ := exec.Command("ls", "-lR", dest).Output()
	t.Log(string(o))

	// check symlink
	fi, err := os.Lstat(filepath.Join(dest, "hello.symlink"))
	if err != nil {
		t.Error("failed to stat hello.symlink", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Error("hello.symlink must be symlink", fi.Mode())
	}
	linkTarget, err := os.Readlink(filepath.Join(dest, "hello.symlink"))
	if err != nil {
		t.Error("failed to readlink hello.symlink", err)
	}
	if diff := cmp.Diff(linkTarget, "hello.txt"); diff != "" {
		t.Errorf("unexpected symlink target %s", diff)
	}
}

type testZipEntry struct {
	name    string
	body    string
	symlink bool
}

func createTestZip(t *testing.T, entries []testZipEntry) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	w := zip.NewWriter(f)
	for _, e := range entries {
		h := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		if e.symlink {
			h.SetMode(os.ModeSymlink | 0777)
		} else {
			h.SetMode(0644)
		}
		fw, err := w.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(e.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestUnzipPathTraversal(t *testing.T) {
	cases := []struct {
		name    string
		entries []testZipEntry
		// prepare is called with dest before unzipping
		prepare func(t *testing.T, dest string)
	}{
		{
			name:    "parent directory",
			entries: []testZipEntry{{name: "../escaped.txt", body: "escaped"}},
		},
		{
			name:    "nested parent directory",
			entries: []testZipEntry{{name: "a/../../escaped.txt", body: "escaped"}},
		},
		{
			name:    "absolute path",
			entries: []testZipEntry{{name: "/tmp/escaped.txt", body: "escaped"}},
		},
		{
			name:    "file via pre-existing symlink",
			entries: []testZipEntry{{name: "link/escaped.txt", body: "escaped"}},
			prepare: func(t *testing.T, dest string) {
				if err := os.Symlink("..", filepath.Join(dest, "link")); err != nil {
					t.Fatal(err)
				}
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.TODO()
			src := createTestZip(t, c.entries)
			base := t.TempDir()
			dest := filepath.Join(base, "dest")
			if err := os.MkdirAll(dest, 0755); err != nil {
				t.Fatal(err)
			}
			if c.prepare != nil {
				c.prepare(t, dest)
			}
			if err := lambroll.Unzip(ctx, src, dest, true); err == nil {
				t.Error("Unzip must fail")
			} else {
				t.Log(err)
			}
			if _, err := os.Stat(filepath.Join(base, "escaped.txt")); err == nil {
				t.Error("file is created outside of dest")
			}
		})
	}
}

func TestUnzipSkipEscapingSymlink(t *testing.T) {
	ctx := context.TODO()
	src := createTestZip(t, []testZipEntry{
		{name: "parent", body: "..", symlink: true},
		{name: "abs", body: "/opt/nodejs/node_modules", symlink: true},
		{name: "dir/link", body: "../..", symlink: true},
		{name: "dir/link/escaped.txt", body: "escaped"},
		{name: "hello.txt", body: "hello"},
	})
	base := t.TempDir()
	dest := filepath.Join(base, "dest")
	if err := lambroll.Unzip(ctx, src, dest, true); err != nil {
		t.Fatal("failed to Unzip", err)
	}
	for _, name := range []string{"parent", "abs"} {
		if _, err := os.Lstat(filepath.Join(dest, name)); err == nil {
			t.Errorf("symlink %s must not be created", name)
		}
	}
	// dir/link is not created as a symlink, so dir/link/escaped.txt is extracted inside dest
	fi, err := os.Lstat(filepath.Join(dest, "dir/link"))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		t.Error("dir/link must not be a symlink")
	}
	if _, err := os.Stat(filepath.Join(base, "escaped.txt")); err == nil {
		t.Error("file is created outside of dest")
	}
	if b, err := os.ReadFile(filepath.Join(dest, "hello.txt")); err != nil || string(b) != "hello" {
		t.Errorf("hello.txt must be extracted: %q %v", string(b), err)
	}
}

func TestUnzipSymlinkInDest(t *testing.T) {
	ctx := context.TODO()
	src := createTestZip(t, []testZipEntry{
		{name: "dir/file.txt", body: "hello"},
		{name: "dir/sub/link.txt", body: "../file.txt", symlink: true},
		{name: "dirlink", body: "dir", symlink: true},
		{name: "dirlink/via-link.txt", body: "world"},
		{name: "a/../b.txt", body: "b"},
	})
	dest := t.TempDir()
	if err := lambroll.Unzip(ctx, src, dest, true); err != nil {
		t.Fatal("failed to Unzip", err)
	}
	for path, expected := range map[string]string{
		"dir/sub/link.txt": "hello",
		"dir/via-link.txt": "world",
		"b.txt":            "b",
	} {
		b, err := os.ReadFile(filepath.Join(dest, path))
		if err != nil {
			t.Error(err)
			continue
		}
		if string(b) != expected {
			t.Errorf("unexpected content of %s: %q", path, string(b))
		}
	}
}

func TestUnzipOverwriteTruncates(t *testing.T) {
	ctx := context.TODO()
	src := createTestZip(t, []testZipEntry{{name: "file.txt", body: "short"}})
	dest := t.TempDir()
	if err := os.WriteFile(filepath.Join(dest, "file.txt"), []byte("long long content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := lambroll.Unzip(ctx, src, dest, true); err != nil {
		t.Fatal("failed to Unzip", err)
	}
	b, err := os.ReadFile(filepath.Join(dest, "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "short" {
		t.Errorf("unexpected content %q", string(b))
	}
}
