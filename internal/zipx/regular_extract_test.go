package zipx_test

import (
	"archive/zip"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bodrovis/lokex/v2/internal/zipx"
)

type errCloser struct {
	err error
}

func (c errCloser) Close() error {
	return c.err
}

func makeZipWithEntryAndModified(t *testing.T, zipPath, name string, data []byte, modified time.Time) {
	t.Helper()

	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = f.Close()
	}()

	zw := zip.NewWriter(f)

	h := &zip.FileHeader{
		Name:     name,
		Method:   zip.Store,
		Modified: modified,
	}
	w, err := zw.CreateHeader(h)
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

func TestExtractRegularFileEntry(t *testing.T) {
	t.Run("file open error", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		zipPath := filepath.Join(tmpDir, "bad.zip")
		makeZipWithUnsupportedMethod(t, zipPath, "a.txt", []byte("abc"))

		zr, err := zip.OpenReader(zipPath)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = zr.Close()
		}()

		_, err = zipx.ExportExtractRegularFileEntry(
			zr.File[0],
			filepath.Join(tmpDir, "out.txt"),
			zipx.DefaultPolicy(),
		)
		if err == nil {
			t.Fatal("ExtractRegularFileEntry() error = nil, want non-nil")
		}
	})

	t.Run("create temp output file error closes zip entry reader", func(t *testing.T) {
		restore := zipx.ExportSetCreateTempFileForTest(func(string, string) (*os.File, error) {
			return nil, errors.New("mktemp boom")
		})
		defer restore()

		tmpDir := t.TempDir()
		zipPath := filepath.Join(tmpDir, "a.zip")
		makeZipWithEntryAndModified(t, zipPath, "a.txt", []byte("abc"), time.Now())

		zr, err := zip.OpenReader(zipPath)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = zr.Close()
		}()

		_, err = zipx.ExportExtractRegularFileEntry(zr.File[0], filepath.Join(tmpDir, "out.txt"), zipx.DefaultPolicy())
		if err == nil {
			t.Fatal("ExtractRegularFileEntry() error = nil, want non-nil")
		}
		if err.Error() != "mktemp boom" {
			t.Fatalf("error = %q, want %q", err.Error(), "mktemp boom")
		}
	})

	t.Run("copy capped error removes temp file", func(t *testing.T) {
		removed := ""
		restoreRemove := zipx.ExportSetRemoveFileForTest(func(name string) error {
			removed = name
			return nil
		})
		defer restoreRemove()

		tmpDir := t.TempDir()
		zipPath := filepath.Join(tmpDir, "a.zip")
		makeZipWithEntryAndModified(t, zipPath, "a.txt", []byte("abcd"), time.Now())

		zr, err := zip.OpenReader(zipPath)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = zr.Close()
		}()

		_, err = zipx.ExportExtractRegularFileEntry(
			zr.File[0],
			filepath.Join(tmpDir, "out.txt"),
			zipx.Policy{MaxFileBytes: 3},
		)
		if err == nil {
			t.Fatal("ExtractRegularFileEntry() error = nil, want non-nil")
		}
		if removed == "" {
			t.Fatal("temp file was not removed on write error")
		}
	})

	t.Run("finalize extracted file error is returned", func(t *testing.T) {
		restoreRename := zipx.ExportSetRenameFileForTest(func(string, string) error {
			return errors.New("rename boom")
		})
		defer restoreRename()

		tmpDir := t.TempDir()
		zipPath := filepath.Join(tmpDir, "a.zip")
		makeZipWithEntryAndModified(t, zipPath, "a.txt", []byte("abc"), time.Now())

		zr, err := zip.OpenReader(zipPath)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = zr.Close()
		}()

		_, err = zipx.ExportExtractRegularFileEntry(
			zr.File[0],
			filepath.Join(tmpDir, "out.txt"),
			zipx.DefaultPolicy(),
		)
		if err == nil {
			t.Fatal("ExtractRegularFileEntry() error = nil, want non-nil")
		}
		if err.Error() != "rename boom" {
			t.Fatalf("error = %q, want %q", err.Error(), "rename boom")
		}
	})
}

func TestFilePermOrDefault(t *testing.T) {
	t.Parallel()

	if got := zipx.ExportFilePermOrDefault(0); got != 0o644 {
		t.Fatalf("perm = %v, want %v", got, os.FileMode(0o644))
	}

	if got := zipx.ExportFilePermOrDefault(0o600); got != 0o600 {
		t.Fatalf("perm = %v, want %v", got, os.FileMode(0o600))
	}
}

func TestCreateTempOutputFile(t *testing.T) {
	t.Run("create temp file error", func(t *testing.T) {
		restore := zipx.ExportSetCreateTempFileForTest(func(string, string) (*os.File, error) {
			return nil, errors.New("mktemp boom")
		})
		defer restore()

		f, tmp, err := zipx.ExportCreateTempOutputFile(filepath.Join(t.TempDir(), "out.txt"), 0o644)
		if err == nil {
			t.Fatal("CreateTempOutputFile() error = nil, want non-nil")
		}
		if err.Error() != "mktemp boom" {
			t.Fatalf("error = %q, want %q", err.Error(), "mktemp boom")
		}
		if f != nil {
			t.Fatal("file != nil, want nil on error")
		}
		if tmp != "" {
			t.Fatalf("tmp = %q, want empty string", tmp)
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		f, tmp, err := zipx.ExportCreateTempOutputFile(filepath.Join(t.TempDir(), "out.txt"), 0o644)
		if err != nil {
			t.Fatalf("CreateTempOutputFile() unexpected error = %v", err)
		}
		if f == nil {
			t.Fatal("file = nil, want non-nil")
		}
		if tmp == "" {
			t.Fatal("tmp = empty, want non-empty")
		}
		_ = f.Close()
		_ = os.Remove(tmp)
	})
}

func TestCloseWithPrecedence(t *testing.T) {
	t.Parallel()

	t.Run("nil closers are skipped", func(t *testing.T) {
		t.Parallel()

		err := zipx.ExportCloseWithPrecedence(nil, nil)
		if err != nil {
			t.Fatalf("CloseWithPrecedence() unexpected error = %v", err)
		}
	})

	t.Run("first close error wins when current nil", func(t *testing.T) {
		t.Parallel()

		err := zipx.ExportCloseWithPrecedence(nil,
			errCloser{err: errors.New("close boom")},
			errCloser{err: errors.New("later boom")},
		)
		if err == nil {
			t.Fatal("CloseWithPrecedence() error = nil, want non-nil")
		}
		if err.Error() != "close boom" {
			t.Fatalf("error = %q, want %q", err.Error(), "close boom")
		}
	})

	t.Run("current error has precedence", func(t *testing.T) {
		t.Parallel()

		err := zipx.ExportCloseWithPrecedence(errors.New("current boom"),
			errCloser{err: errors.New("close boom")},
		)
		if err == nil {
			t.Fatal("CloseWithPrecedence() error = nil, want non-nil")
		}
		if err.Error() != "current boom" {
			t.Fatalf("error = %q, want %q", err.Error(), "current boom")
		}
	})
}

func TestFinalizeExtractedFile(t *testing.T) {
	t.Run("rename error removes temp file", func(t *testing.T) {
		var removed []string

		restoreRename := zipx.ExportSetRenameFileForTest(func(string, string) error {
			return errors.New("rename boom")
		})
		defer restoreRename()

		restoreRemove := zipx.ExportSetRemoveFileForTest(func(name string) error {
			removed = append(removed, name)
			return nil
		})
		defer restoreRemove()

		err := zipx.ExportFinalizeExtractedFile("tmp-file", "target-file", time.Time{}, false)
		if err == nil {
			t.Fatal("FinalizeExtractedFile() error = nil, want non-nil")
		}
		if err.Error() != "rename boom" {
			t.Fatalf("error = %q, want %q", err.Error(), "rename boom")
		}

		if len(removed) != 2 {
			t.Fatalf("remove calls = %v, want [target-file tmp-file]", removed)
		}
		if removed[0] != "target-file" || removed[1] != "tmp-file" {
			t.Fatalf("remove calls = %v, want [target-file tmp-file]", removed)
		}
	})

	t.Run("preserve times calls chtimes", func(t *testing.T) {
		called := false
		restoreRename := zipx.ExportSetRenameFileForTest(func(string, string) error {
			return nil
		})
		defer restoreRename()

		restoreRemove := zipx.ExportSetRemoveFileForTest(func(string) error {
			return nil
		})
		defer restoreRemove()

		restoreChtimes := zipx.ExportSetChtimesFileForTest(func(string, time.Time, time.Time) error {
			called = true
			return nil
		})
		defer restoreChtimes()

		modified := time.Now()
		err := zipx.ExportFinalizeExtractedFile("tmp-file", "target-file", modified, true)
		if err != nil {
			t.Fatalf("FinalizeExtractedFile() unexpected error = %v", err)
		}
		if !called {
			t.Fatal("chtimes was not called")
		}
	})

	t.Run("zero modified or preserve false skips chtimes", func(t *testing.T) {
		called := false
		restoreRename := zipx.ExportSetRenameFileForTest(func(string, string) error {
			return nil
		})
		defer restoreRename()

		restoreRemove := zipx.ExportSetRemoveFileForTest(func(string) error {
			return nil
		})
		defer restoreRemove()

		restoreChtimes := zipx.ExportSetChtimesFileForTest(func(string, time.Time, time.Time) error {
			called = true
			return nil
		})
		defer restoreChtimes()

		err := zipx.ExportFinalizeExtractedFile("tmp-file", "target-file", time.Time{}, true)
		if err != nil {
			t.Fatalf("FinalizeExtractedFile() unexpected error = %v", err)
		}
		if called {
			t.Fatal("chtimes was called, want skipped")
		}
	})
}

func makeZipWithUnsupportedMethod(t *testing.T, zipPath, name string, data []byte) {
	t.Helper()

	makeZipWithEntryAndModified(t, zipPath, name, data, time.Now())

	b, err := os.ReadFile(zipPath)
	if err != nil {
		t.Fatal(err)
	}

	const (
		localHeaderSig   = 0x04034b50
		centralHeaderSig = 0x02014b50
		badMethod        = uint16(99)
	)

	// Patch local file header method at offset 8.
	for i := 0; i+4 <= len(b); i++ {
		if uint32(b[i])|
			uint32(b[i+1])<<8|
			uint32(b[i+2])<<16|
			uint32(b[i+3])<<24 == localHeaderSig {
			// compression method is at offset 8 from local header start
			if i+10 > len(b) {
				t.Fatal("truncated local header")
			}
			b[i+8] = byte(badMethod)
			b[i+9] = byte(badMethod >> 8)
			break
		}
	}

	// Patch central directory header method at offset 10.
	for i := 0; i+4 <= len(b); i++ {
		if uint32(b[i])|
			uint32(b[i+1])<<8|
			uint32(b[i+2])<<16|
			uint32(b[i+3])<<24 == centralHeaderSig {
			// compression method is at offset 10 from central header start
			if i+12 > len(b) {
				t.Fatal("truncated central header")
			}
			b[i+10] = byte(badMethod)
			b[i+11] = byte(badMethod >> 8)
			break
		}
	}

	if err := os.WriteFile(zipPath, b, 0o644); err != nil {
		t.Fatal(err)
	}
}
