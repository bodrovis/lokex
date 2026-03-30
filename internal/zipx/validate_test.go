package zipx_test

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/bodrovis/lokex/v2/internal/zipx"
)

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
