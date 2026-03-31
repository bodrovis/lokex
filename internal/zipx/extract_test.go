package zipx_test

import (
	"archive/zip"
	"errors"
	"os"
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

func TestUnzip(t *testing.T) {
	t.Run("open zip reader error", func(t *testing.T) {
		restore := zipx.ExportSetOpenZipReaderForTest(func(string) (zipx.ExportZipReader, error) {
			return nil, errors.New("open boom")
		})
		defer restore()

		err := zipx.Unzip("/tmp/x.zip", t.TempDir(), zipx.DefaultPolicy())
		if err == nil {
			t.Fatal("Unzip() error = nil, want non-nil")
		}
		if err.Error() != "open boom" {
			t.Fatalf("error = %q, want %q", err.Error(), "open boom")
		}
	})

	t.Run("close zip error is returned", func(t *testing.T) {
		restoreOpen := zipx.ExportSetOpenZipReaderForTest(func(string) (zipx.ExportZipReader, error) {
			return fakeZipReader{
				files:    nil,
				closeErr: errors.New("close boom"),
			}, nil
		})
		defer restoreOpen()

		restoreMkdir := zipx.ExportSetMkdirAllForTest(func(string, os.FileMode) error {
			return nil
		})
		defer restoreMkdir()

		restoreAbs := zipx.ExportSetAbsPathForTest(func(path string) (string, error) {
			return path, nil
		})
		defer restoreAbs()

		restoreEval := zipx.ExportSetEvalSymlinksForTest(func(path string) (string, error) {
			return "", errors.New("ignore")
		})
		defer restoreEval()

		err := zipx.Unzip("/tmp/x.zip", t.TempDir(), zipx.DefaultPolicy())
		if err == nil {
			t.Fatal("Unzip() error = nil, want non-nil")
		}
		if err.Error() != "close zip: close boom" {
			t.Fatalf("error = %q, want %q", err.Error(), "close zip: close boom")
		}
	})
}

func TestPrepareExtractionRoot(t *testing.T) {
	t.Run("mkdir all error", func(t *testing.T) {
		restore := zipx.ExportSetMkdirAllForTest(func(string, os.FileMode) error {
			return errors.New("mkdir boom")
		})
		defer restore()

		got, err := zipx.ExportPrepareExtractionRoot("/tmp/x")
		if err == nil {
			t.Fatal("PrepareExtractionRoot() error = nil, want non-nil")
		}
		if err.Error() != "mkdir boom" {
			t.Fatalf("error = %q, want %q", err.Error(), "mkdir boom")
		}
		if got != "" {
			t.Fatalf("got = %q, want empty string on error", got)
		}
	})

	t.Run("abs error", func(t *testing.T) {
		restoreMkdir := zipx.ExportSetMkdirAllForTest(func(string, os.FileMode) error {
			return nil
		})
		defer restoreMkdir()

		restoreAbs := zipx.ExportSetAbsPathForTest(func(string) (string, error) {
			return "", errors.New("abs boom")
		})
		defer restoreAbs()

		got, err := zipx.ExportPrepareExtractionRoot("/tmp/x")
		if err == nil {
			t.Fatal("PrepareExtractionRoot() error = nil, want non-nil")
		}
		if err.Error() != "abs boom" {
			t.Fatalf("error = %q, want %q", err.Error(), "abs boom")
		}
		if got != "" {
			t.Fatalf("got = %q, want empty string on error", got)
		}
	})
}

func TestUnzip_OpenZipReaderError_Default(t *testing.T) {
	t.Parallel()

	err := zipx.Unzip("/definitely/not/exist.zip", t.TempDir(), zipx.DefaultPolicy())
	if err == nil {
		t.Fatal("Unzip() error = nil, want non-nil")
	}
}
