package zipx_test

import (
	"archive/zip"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/bodrovis/lokex/v2/internal/zipx"
)

func TestExtractEntry(t *testing.T) {
	t.Run("check parent symlinks error is returned", func(t *testing.T) {
		restore := zipx.ExportSetPathHasSymlinkOutsideForTest(func(string, string) (bool, error) {
			return false, errors.New("symlink boom")
		})
		defer restore()

		tmpDir := t.TempDir()
		zipPath := filepath.Join(tmpDir, "test.zip")
		makeZipWithEntry(t, zipPath, "file.txt", []byte("abc"))

		zr, err := zip.OpenReader(zipPath)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = zr.Close()
		}()

		_, err = zipx.ExportExtractEntry(zr.File[0], tmpDir, tmpDir, zipx.DefaultPolicy())
		if err == nil {
			t.Fatal("ExtractEntry() error = nil, want non-nil")
		}
		if err.Error() != "symlink boom" {
			t.Fatalf("error = %q, want %q", err.Error(), "symlink boom")
		}
	})

	t.Run("special file mode is skipped", func(t *testing.T) {
		restore := zipx.ExportSetPathHasSymlinkOutsideForTest(func(string, string) (bool, error) {
			return false, nil
		})
		defer restore()

		destDir := t.TempDir()

		f := &zip.File{
			FileHeader: zip.FileHeader{
				Name:   "pipe",
				Method: zip.Store,
			},
		}
		f.SetMode(os.ModeNamedPipe)

		n, err := zipx.ExportExtractEntry(f, destDir, destDir, zipx.DefaultPolicy())
		if err != nil {
			t.Fatalf("ExtractEntry() unexpected error = %v", err)
		}
		if n != 0 {
			t.Fatalf("n = %d, want %d", n, 0)
		}
	})
}

func TestPrepareEntryTarget(t *testing.T) {
	t.Run("empty normalized path is skipped", func(t *testing.T) {
		f := &zip.File{
			FileHeader: zip.FileHeader{
				Name: "/",
			},
		}

		targetAbs, info, mode, skip, err := zipx.ExportPrepareEntryTarget(
			f,
			t.TempDir(),
			t.TempDir(),
			zipx.DefaultPolicy(),
		)
		if err != nil {
			t.Fatalf("PrepareEntryTarget() unexpected error = %v", err)
		}
		if targetAbs != "" {
			t.Fatalf("targetAbs = %q, want empty", targetAbs)
		}
		if info != nil {
			t.Fatalf("info = %#v, want nil", info)
		}
		if mode != 0 {
			t.Fatalf("mode = %v, want 0", mode)
		}
		if !skip {
			t.Fatal("skip = false, want true")
		}
	})

	t.Run("resolve target path error is returned", func(t *testing.T) {
		destDir := t.TempDir()
		destReal := t.TempDir() // intentionally different root

		f := &zip.File{
			FileHeader: zip.FileHeader{
				Name: "file.txt",
			},
		}

		_, _, _, skip, err := zipx.ExportPrepareEntryTarget(
			f,
			destDir,
			destReal,
			zipx.DefaultPolicy(),
		)
		if err == nil {
			t.Fatal("PrepareEntryTarget() error = nil, want non-nil")
		}
		if skip {
			t.Fatal("skip = true, want false")
		}
	})

	t.Run("resolve target path error is returned", func(t *testing.T) {
		f := &zip.File{
			FileHeader: zip.FileHeader{
				Name: "../evil.txt",
			},
		}

		_, _, _, skip, err := zipx.ExportPrepareEntryTarget(
			f,
			t.TempDir(),
			t.TempDir(),
			zipx.DefaultPolicy(),
		)
		if err == nil {
			t.Fatal("PrepareEntryTarget() error = nil, want non-nil")
		}
		if skip {
			t.Fatal("skip = true, want false")
		}
	})
}

func TestExtractDirEntry(t *testing.T) {
	t.Run("mkdir all error is returned", func(t *testing.T) {
		restore := zipx.ExportSetMkdirAllDirForTest(func(string, os.FileMode) error {
			return errors.New("mkdir boom")
		})
		defer restore()

		f := &zip.File{
			FileHeader: zip.FileHeader{
				Name: "dir/",
			},
		}

		err := zipx.ExportExtractDirEntry(f, filepath.Join(t.TempDir(), "dir"), zipx.DefaultPolicy())
		if err == nil {
			t.Fatal("ExtractDirEntry() error = nil, want non-nil")
		}
		if err.Error() != "mkdir boom" {
			t.Fatalf("error = %q, want %q", err.Error(), "mkdir boom")
		}
	})
}

func TestCheckParentSymlinks(t *testing.T) {
	t.Run("unsafe parent symlink returns error", func(t *testing.T) {
		restore := zipx.ExportSetPathHasSymlinkOutsideForTest(func(string, string) (bool, error) {
			return true, nil
		})
		defer restore()

		err := zipx.ExportCheckParentSymlinks("/dest", "/dest/file.txt", "file.txt")
		if err == nil {
			t.Fatal("CheckParentSymlinks() error = nil, want non-nil")
		}
		if err.Error() != `unsafe symlink in parents for: "file.txt"` {
			t.Fatalf("error = %q, want %q", err.Error(), `unsafe symlink in parents for: "file.txt"`)
		}
	})

	t.Run("non not-exist error is returned", func(t *testing.T) {
		restore := zipx.ExportSetPathHasSymlinkOutsideForTest(func(string, string) (bool, error) {
			return false, errors.New("path boom")
		})
		defer restore()

		err := zipx.ExportCheckParentSymlinks("/dest", "/dest/file.txt", "file.txt")
		if err == nil {
			t.Fatal("CheckParentSymlinks() error = nil, want non-nil")
		}
		if err.Error() != "path boom" {
			t.Fatalf("error = %q, want %q", err.Error(), "path boom")
		}
	})
}

func makeZipWithEntry(t *testing.T, zipPath, name string, data []byte) {
	t.Helper()

	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = f.Close()
	}()

	zw := zip.NewWriter(f)
	w, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}
