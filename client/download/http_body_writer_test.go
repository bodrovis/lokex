package download_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bodrovis/lokex/v2/client/download"
)

type errReader struct {
	err error
}

func (r errReader) Read(_ []byte) (int, error) {
	return 0, r.err
}

func TestWriteHTTPBodyAtomically_SuccessExactLength(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dest := filepath.Join(dir, "bundle.zip")
	body := []byte("hello zip")

	err := download.ExportWriteHTTPBodyAtomically(dest, bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("writeHTTPBodyAtomically() error = %v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != string(body) {
		t.Fatalf("file content = %q, want %q", string(got), string(body))
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".part-") {
			t.Fatalf("temp file %q was not cleaned up", e.Name())
		}
	}
}

func TestWriteHTTPBodyAtomically_SuccessWhenWantLenDisabled(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dest := filepath.Join(dir, "bundle.zip")
	body := []byte("hello zip")

	err := download.ExportWriteHTTPBodyAtomically(dest, bytes.NewReader(body), -1)
	if err != nil {
		t.Fatalf("writeHTTPBodyAtomically() error = %v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != string(body) {
		t.Fatalf("file content = %q, want %q", string(got), string(body))
	}
}

func TestWriteHTTPBodyAtomically_CreateTempError(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	missingDir := filepath.Join(base, "missing")
	dest := filepath.Join(missingDir, "bundle.zip")

	err := download.ExportWriteHTTPBodyAtomically(dest, bytes.NewReader([]byte("x")), 1)
	if err == nil {
		t.Fatal("writeHTTPBodyAtomically() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "create temp zip:") {
		t.Fatalf("error = %q, want prefix containing %q", err.Error(), "create temp zip:")
	}
}

func TestWriteHTTPBodyAtomically_CopyErrorRemovesTempFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dest := filepath.Join(dir, "bundle.zip")

	wantErr := errors.New("boom read")
	err := download.ExportWriteHTTPBodyAtomically(dest, errReader{err: wantErr}, -1)
	if err == nil {
		t.Fatal("writeHTTPBodyAtomically() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "write zip:") {
		t.Fatalf("error = %q, want prefix containing %q", err.Error(), "write zip:")
	}
	if !strings.Contains(err.Error(), wantErr.Error()) {
		t.Fatalf("error = %q, want wrapped error containing %q", err.Error(), wantErr.Error())
	}

	if _, statErr := os.Stat(dest); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("dest stat error = %v, want %v", statErr, os.ErrNotExist)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".part-") {
			t.Fatalf("temp file %q was not cleaned up", e.Name())
		}
	}
}

func TestWriteHTTPBodyAtomically_LengthMismatchRemovesTempFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dest := filepath.Join(dir, "bundle.zip")
	body := []byte("short")

	err := download.ExportWriteHTTPBodyAtomically(dest, bytes.NewReader(body), int64(len(body)+10))
	if err == nil {
		t.Fatal("writeHTTPBodyAtomically() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "incomplete download:") {
		t.Fatalf("error = %q, want prefix containing %q", err.Error(), "incomplete download:")
	}
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("error = %v, want wrapped %v", err, io.ErrUnexpectedEOF)
	}

	if _, statErr := os.Stat(dest); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("dest stat error = %v, want %v", statErr, os.ErrNotExist)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".part-") {
			t.Fatalf("temp file %q was not cleaned up", e.Name())
		}
	}
}

func TestWriteHTTPBodyAtomically_ReplacesExistingDestination(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dest := filepath.Join(dir, "bundle.zip")

	if err := os.WriteFile(dest, []byte("old data"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	newBody := []byte("new data")
	err := download.ExportWriteHTTPBodyAtomically(dest, bytes.NewReader(newBody), int64(len(newBody)))
	if err != nil {
		t.Fatalf("writeHTTPBodyAtomically() error = %v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != string(newBody) {
		t.Fatalf("file content = %q, want %q", string(got), string(newBody))
	}
}
