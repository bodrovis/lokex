package zipx_test

import (
	"archive/zip"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/bodrovis/lokex/v2/internal/zipx"
)

type fakeZipReader struct {
	files    []*zip.File
	closeErr error
}

func (r fakeZipReader) Close() error {
	return r.closeErr
}

func (r fakeZipReader) Files() []*zip.File {
	return r.files
}

func TestPrepareExtractionRoot(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		destDir := filepath.Join(t.TempDir(), "nested", "dest")

		root, err := zipx.ExportPrepareExtractionRoot(destDir)
		if err != nil {
			t.Fatalf(
				"PrepareExtractionRoot() unexpected error = %v",
				err,
			)
		}
		defer func() {
			_ = root.Close()
		}()

		if root == nil {
			t.Fatal("root = nil, want non-nil")
		}

		info, err := os.Stat(destDir)
		if err != nil {
			t.Fatal(err)
		}

		if !info.IsDir() {
			t.Fatalf("%q is not a directory", destDir)
		}
	})

	t.Run("mkdir error", func(t *testing.T) {
		t.Parallel()

		base := t.TempDir()

		file := filepath.Join(base, "file")
		if err := os.WriteFile(
			file,
			[]byte("x"),
			0o644,
		); err != nil {
			t.Fatal(err)
		}

		root, err := zipx.ExportPrepareExtractionRoot(
			filepath.Join(file, "child"),
		)
		if err == nil {
			if root != nil {
				_ = root.Close()
			}

			t.Fatal("PrepareExtractionRoot() error = nil, want non-nil")
		}

		if root != nil {
			t.Fatal("root != nil on error")
		}
	})
}

func TestUnzip_CloseZipErrorIsReturned(t *testing.T) {
	restore := zipx.ExportSetOpenZipReaderForTest(
		func(string) (zipx.ExportZipReader, error) {
			return fakeZipReader{
				closeErr: errors.New("close boom"),
			}, nil
		},
	)
	defer restore()

	err := zipx.Unzip(
		"/unused/test.zip",
		t.TempDir(),
		zipx.DefaultPolicy(),
	)
	if err == nil {
		t.Fatal("Unzip() error = nil, want non-nil")
	}

	if err.Error() != "close zip: close boom" {
		t.Fatalf(
			"error = %q, want %q",
			err.Error(),
			"close zip: close boom",
		)
	}
}

func TestUnzip_OpenZipReaderError_Default(t *testing.T) {
	t.Parallel()

	err := zipx.Unzip(
		"/definitely/not/exist.zip",
		t.TempDir(),
		zipx.DefaultPolicy(),
	)
	if err == nil {
		t.Fatal("Unzip() error = nil, want non-nil")
	}
}
