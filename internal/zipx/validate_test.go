package zipx_test

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bodrovis/lokex/v2/internal/zipx"
)

func TestValidate(t *testing.T) {
	t.Run("open error is wrapped", func(t *testing.T) {
		restore := zipx.ExportSetOpenZipReaderForTest(func(string) (zipx.ExportZipReader, error) {
			return nil, errors.New("open boom")
		})
		defer restore()

		err := zipx.Validate("any.zip")
		if err == nil {
			t.Fatal("Validate() error = nil, want non-nil")
		}
		if err.Error() != "zip validate open: open boom" {
			t.Fatalf("error = %q, want %q", err.Error(), "zip validate open: open boom")
		}
	})

	t.Run("close error is wrapped", func(t *testing.T) {
		restore := zipx.ExportSetOpenZipReaderForTest(func(string) (zipx.ExportZipReader, error) {
			return fakeZipReader{closeErr: errors.New("close boom")}, nil
		})
		defer restore()

		err := zipx.Validate("any.zip")
		if err == nil {
			t.Fatal("Validate() error = nil, want non-nil")
		}
		if err.Error() != "zip validate close: close boom" {
			t.Fatalf("error = %q, want %q", err.Error(), "zip validate close: close boom")
		}
	})
}

func TestValidate_OK(t *testing.T) {
	zp := makeZip(t, []zentry{{name: "a.txt", data: []byte("hi")}})
	if err := zipx.Validate(zp); err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}
}

func TestValidate_NotZip(t *testing.T) {
	fn := filepath.Join(t.TempDir(), "not.zip")
	if err := os.WriteFile(fn, []byte("definitely not a zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := zipx.Validate(fn)
	if err == nil {
		t.Fatalf("Validate() expected error, got nil")
	}
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("expected io.ErrUnexpectedEOF, got: %v", err)
	}
}

func TestValidate_OpenError(t *testing.T) {
	t.Parallel()

	err := zipx.Validate("/definitely/not/exist.zip")
	if err == nil {
		t.Fatal("Validate() error = nil, want non-nil")
	}

	if !strings.Contains(err.Error(), "zip validate open:") {
		t.Fatalf("error = %q, want open error", err.Error())
	}
}
