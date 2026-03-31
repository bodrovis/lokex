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

type testSyncCloseFile struct {
	buf      bytes.Buffer
	name     string
	syncErr  error
	closeErr error
	closed   bool
}

func (f *testSyncCloseFile) Write(p []byte) (int, error) {
	return f.buf.Write(p)
}

func (f *testSyncCloseFile) Sync() error {
	return f.syncErr
}

func (f *testSyncCloseFile) Close() error {
	f.closed = true
	return f.closeErr
}

func (f *testSyncCloseFile) Name() string {
	return f.name
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

func TestCopyAndValidate(t *testing.T) {
	tests := []struct {
		name    string
		tmp     *testSyncCloseFile
		src     io.Reader
		wantLen int64
		wantErr string
	}{
		{
			name: "sync error",
			tmp: &testSyncCloseFile{
				name:    "tmp.zip.part",
				syncErr: errors.New("sync boom"),
			},
			src:     strings.NewReader("abc"),
			wantLen: -1,
			wantErr: "sync zip: sync boom",
		},
		{
			name: "want len mismatch",
			tmp: &testSyncCloseFile{
				name: "tmp.zip.part",
			},
			src:     strings.NewReader("abc"),
			wantLen: 10,
			wantErr: "incomplete download: got 3 of 10: unexpected EOF",
		},
		{
			name: "success",
			tmp: &testSyncCloseFile{
				name: "tmp.zip.part",
			},
			src:     strings.NewReader("abc"),
			wantLen: 3,
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := download.ExportCopyAndValidate(tt.tmp, tt.src, tt.wantLen)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("CopyAndValidate() error = nil, want %q", tt.wantErr)
				}
				if err.Error() != tt.wantErr {
					t.Fatalf("error = %q, want %q", err.Error(), tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("CopyAndValidate() unexpected error = %v", err)
			}
		})
	}
}

func TestFinalizeAtomicWrite(t *testing.T) {
	t.Run("close error", func(t *testing.T) {
		tmp := &testSyncCloseFile{
			name:     "tmp.zip.part",
			closeErr: errors.New("close boom"),
		}
		closed := false

		err := download.ExportFinalizeAtomicWrite(tmp, tmp.Name(), "dest.zip", &closed)
		if err == nil {
			t.Fatal("FinalizeAtomicWrite() error = nil, want non-nil")
		}
		if err.Error() != "close zip: close boom" {
			t.Fatalf("error = %q, want %q", err.Error(), "close zip: close boom")
		}
		if !tmp.closed {
			t.Fatal("tmp file was not closed")
		}
		if closed {
			t.Fatal("closed flag = true, want false on close error")
		}
	})

	t.Run("rename error", func(t *testing.T) {
		restore := download.ExportSetRenameFileForTest(func(oldpath, newpath string) error {
			return errors.New("rename boom")
		})
		defer restore()

		tmp := &testSyncCloseFile{
			name: "tmp.zip.part",
		}
		closed := false

		err := download.ExportFinalizeAtomicWrite(tmp, tmp.Name(), "dest.zip", &closed)
		if err == nil {
			t.Fatal("FinalizeAtomicWrite() error = nil, want non-nil")
		}
		if err.Error() != "finalize zip: rename boom" {
			t.Fatalf("error = %q, want %q", err.Error(), "finalize zip: rename boom")
		}
		if !tmp.closed {
			t.Fatal("tmp file was not closed")
		}
		if !closed {
			t.Fatal("closed flag = false, want true after successful close")
		}
	})
}

func TestWriteHTTPBodyAtomically(t *testing.T) {
	t.Run("finalizeAtomicWrite error is returned", func(t *testing.T) {
		restore := download.ExportSetRenameFileForTest(func(oldpath, newpath string) error {
			return errors.New("rename boom")
		})
		defer restore()

		dir := t.TempDir()
		destPath := filepath.Join(dir, "bundle.zip")

		err := download.ExportWriteHTTPBodyAtomically(destPath, strings.NewReader("payload"), -1)
		if err == nil {
			t.Fatal("WriteHTTPBodyAtomically() error = nil, want non-nil")
		}
		if err.Error() != "finalize zip: rename boom" {
			t.Fatalf("error = %q, want %q", err.Error(), "finalize zip: rename boom")
		}

		_, statErr := os.Stat(destPath)
		if !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("dest file exists or unexpected stat error = %v, want not exist", statErr)
		}
	})
}
