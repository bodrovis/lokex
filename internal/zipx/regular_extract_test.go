package zipx_test

import (
	"archive/zip"
	"errors"
	"os"
	"path/filepath"
	"strings"
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

func makeZipWithEntryAndModified(
	t *testing.T,
	zipPath,
	name string,
	data []byte,
	modified time.Time,
) {
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
	t.Parallel()

	t.Run("file open error", func(t *testing.T) {
		t.Parallel()

		srcDir := t.TempDir()
		destDir := t.TempDir()

		root, err := os.OpenRoot(destDir)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = root.Close()
		}()

		zipPath := filepath.Join(srcDir, "bad.zip")
		makeZipWithUnsupportedMethod(
			t,
			zipPath,
			"a.txt",
			[]byte("abc"),
		)

		zr, err := zip.OpenReader(zipPath)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = zr.Close()
		}()

		_, err = zipx.ExportExtractRegularFileEntry(
			zr.File[0],
			root,
			"out.txt",
			zipx.DefaultPolicy(),
		)
		if err == nil {
			t.Fatal("ExtractRegularFileEntry() error = nil, want non-nil")
		}
	})

	t.Run("create temp output file error is returned", func(t *testing.T) {
		t.Parallel()

		srcDir := t.TempDir()
		destDir := t.TempDir()

		root, err := os.OpenRoot(destDir)
		if err != nil {
			t.Fatal(err)
		}

		// Operations on a closed Root must fail.
		if err := root.Close(); err != nil {
			t.Fatal(err)
		}

		zipPath := filepath.Join(srcDir, "a.zip")
		makeZipWithEntryAndModified(
			t,
			zipPath,
			"a.txt",
			[]byte("abc"),
			time.Now(),
		)

		zr, err := zip.OpenReader(zipPath)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = zr.Close()
		}()

		_, err = zipx.ExportExtractRegularFileEntry(
			zr.File[0],
			root,
			"out.txt",
			zipx.DefaultPolicy(),
		)
		if err == nil {
			t.Fatal("ExtractRegularFileEntry() error = nil, want non-nil")
		}
	})

	t.Run("copy capped error removes temp file", func(t *testing.T) {
		t.Parallel()

		srcDir := t.TempDir()
		destDir := t.TempDir()

		root, err := os.OpenRoot(destDir)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = root.Close()
		}()

		zipPath := filepath.Join(srcDir, "a.zip")
		makeZipWithEntryAndModified(
			t,
			zipPath,
			"a.txt",
			[]byte("abcd"),
			time.Now(),
		)

		zr, err := zip.OpenReader(zipPath)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = zr.Close()
		}()

		_, err = zipx.ExportExtractRegularFileEntry(
			zr.File[0],
			root,
			"out.txt",
			zipx.Policy{MaxFileBytes: 3},
		)
		if err == nil {
			t.Fatal("ExtractRegularFileEntry() error = nil, want non-nil")
		}

		entries, err := os.ReadDir(destDir)
		if err != nil {
			t.Fatal(err)
		}

		if len(entries) != 0 {
			t.Fatalf(
				"destination contains %d entries after failure, want 0",
				len(entries),
			)
		}
	})

	t.Run("finalize error removes temp file", func(t *testing.T) {
		t.Parallel()

		srcDir := t.TempDir()
		destDir := t.TempDir()

		// Rename of a regular file over an existing directory must fail.
		if err := os.Mkdir(
			filepath.Join(destDir, "out.txt"),
			0o755,
		); err != nil {
			t.Fatal(err)
		}

		root, err := os.OpenRoot(destDir)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = root.Close()
		}()

		zipPath := filepath.Join(srcDir, "a.zip")
		makeZipWithEntryAndModified(
			t,
			zipPath,
			"a.txt",
			[]byte("abc"),
			time.Now(),
		)

		zr, err := zip.OpenReader(zipPath)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = zr.Close()
		}()

		_, err = zipx.ExportExtractRegularFileEntry(
			zr.File[0],
			root,
			"out.txt",
			zipx.DefaultPolicy(),
		)
		if err == nil {
			t.Fatal("ExtractRegularFileEntry() error = nil, want non-nil")
		}

		entries, err := os.ReadDir(destDir)
		if err != nil {
			t.Fatal(err)
		}

		if len(entries) != 1 || entries[0].Name() != "out.txt" {
			t.Fatalf(
				"destination entries = %v, want only out.txt directory",
				entries,
			)
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srcDir := t.TempDir()
		destDir := t.TempDir()

		root, err := os.OpenRoot(destDir)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = root.Close()
		}()

		zipPath := filepath.Join(srcDir, "a.zip")
		makeZipWithEntryAndModified(
			t,
			zipPath,
			"a.txt",
			[]byte("abc"),
			time.Now(),
		)

		zr, err := zip.OpenReader(zipPath)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = zr.Close()
		}()

		n, err := zipx.ExportExtractRegularFileEntry(
			zr.File[0],
			root,
			"out.txt",
			zipx.DefaultPolicy(),
		)
		if err != nil {
			t.Fatalf(
				"ExtractRegularFileEntry() unexpected error = %v",
				err,
			)
		}

		if n != 3 {
			t.Fatalf("n = %d, want 3", n)
		}

		got, err := root.ReadFile("out.txt")
		if err != nil {
			t.Fatal(err)
		}

		if string(got) != "abc" {
			t.Fatalf("content = %q, want %q", got, "abc")
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
	t.Parallel()

	t.Run("closed root returns error", func(t *testing.T) {
		t.Parallel()

		root, err := os.OpenRoot(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}

		if err := root.Close(); err != nil {
			t.Fatal(err)
		}

		f, tmp, err := zipx.ExportCreateTempOutputFile(
			root,
			"out.txt",
			0o644,
		)
		if err == nil {
			t.Fatal("CreateTempOutputFile() error = nil, want non-nil")
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

		destDir := t.TempDir()

		root, err := os.OpenRoot(destDir)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = root.Close()
		}()

		f, tmp, err := zipx.ExportCreateTempOutputFile(
			root,
			"out.txt",
			0o644,
		)
		if err != nil {
			t.Fatalf(
				"CreateTempOutputFile() unexpected error = %v",
				err,
			)
		}

		if f == nil {
			t.Fatal("file = nil, want non-nil")
		}
		defer func() {
			_ = f.Close()
		}()

		if tmp == "" {
			t.Fatal("tmp = empty, want non-empty")
		}

		if filepath.IsAbs(tmp) {
			t.Fatalf("tmp = %q, want relative path", tmp)
		}

		if !strings.HasPrefix(
			filepath.Base(tmp),
			"out.txt.partial-",
		) {
			t.Fatalf(
				"tmp = %q, want out.txt.partial-* name",
				tmp,
			)
		}

		if _, err := root.Stat(tmp); err != nil {
			t.Fatalf("Stat(%q) error = %v", tmp, err)
		}

		if err := root.Remove(tmp); err != nil {
			t.Fatal(err)
		}
	})
}

func TestCloseWithPrecedence(t *testing.T) {
	t.Parallel()

	t.Run("nil closers are skipped", func(t *testing.T) {
		t.Parallel()

		err := zipx.ExportCloseWithPrecedence(nil, nil)
		if err != nil {
			t.Fatalf(
				"CloseWithPrecedence() unexpected error = %v",
				err,
			)
		}
	})

	t.Run("first close error wins when current nil", func(t *testing.T) {
		t.Parallel()

		err := zipx.ExportCloseWithPrecedence(
			nil,
			errCloser{err: errors.New("close boom")},
			errCloser{err: errors.New("later boom")},
		)
		if err == nil {
			t.Fatal("CloseWithPrecedence() error = nil, want non-nil")
		}

		if err.Error() != "close boom" {
			t.Fatalf(
				"error = %q, want %q",
				err.Error(),
				"close boom",
			)
		}
	})

	t.Run("current error has precedence", func(t *testing.T) {
		t.Parallel()

		err := zipx.ExportCloseWithPrecedence(
			errors.New("current boom"),
			errCloser{err: errors.New("close boom")},
		)
		if err == nil {
			t.Fatal("CloseWithPrecedence() error = nil, want non-nil")
		}

		if err.Error() != "current boom" {
			t.Fatalf(
				"error = %q, want %q",
				err.Error(),
				"current boom",
			)
		}
	})
}

func TestFinalizeExtractedFile(t *testing.T) {
	t.Parallel()

	t.Run("rename error removes temp file", func(t *testing.T) {
		t.Parallel()

		destDir := t.TempDir()

		root, err := os.OpenRoot(destDir)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = root.Close()
		}()

		if err := root.WriteFile(
			"tmp-file",
			[]byte("data"),
			0o644,
		); err != nil {
			t.Fatal(err)
		}

		// Renaming a file over a directory must fail.
		if err := root.Mkdir("target-dir", 0o755); err != nil {
			t.Fatal(err)
		}

		err = zipx.ExportFinalizeExtractedFile(
			root,
			"tmp-file",
			"target-dir",
			time.Time{},
			false,
		)
		if err == nil {
			t.Fatal("FinalizeExtractedFile() error = nil, want non-nil")
		}

		if _, err := root.Lstat("tmp-file"); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf(
				"temporary file still exists: Lstat() error = %v",
				err,
			)
		}

		info, err := root.Stat("target-dir")
		if err != nil {
			t.Fatal(err)
		}

		if !info.IsDir() {
			t.Fatal("target-dir is no longer a directory")
		}
	})

	t.Run("preserve times applies modified time", func(t *testing.T) {
		t.Parallel()

		destDir := t.TempDir()

		root, err := os.OpenRoot(destDir)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = root.Close()
		}()

		if err := root.WriteFile(
			"tmp-file",
			[]byte("data"),
			0o644,
		); err != nil {
			t.Fatal(err)
		}

		modified := time.Date(
			2020, time.January, 2,
			3, 4, 5, 0,
			time.UTC,
		)

		err = zipx.ExportFinalizeExtractedFile(
			root,
			"tmp-file",
			"target-file",
			modified,
			true,
		)
		if err != nil {
			t.Fatalf(
				"FinalizeExtractedFile() unexpected error = %v",
				err,
			)
		}

		info, err := root.Stat("target-file")
		if err != nil {
			t.Fatal(err)
		}

		if !info.ModTime().Equal(modified) {
			t.Fatalf(
				"mod time = %v, want %v",
				info.ModTime(),
				modified,
			)
		}
	})

	t.Run("preserve false keeps existing modified time", func(t *testing.T) {
		t.Parallel()

		destDir := t.TempDir()

		root, err := os.OpenRoot(destDir)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = root.Close()
		}()

		if err := root.WriteFile(
			"tmp-file",
			[]byte("data"),
			0o644,
		); err != nil {
			t.Fatal(err)
		}

		before := time.Date(
			2020, time.January, 2,
			3, 4, 5, 0,
			time.UTC,
		)
		modified := before.Add(24 * time.Hour)

		if err := root.Chtimes("tmp-file", before, before); err != nil {
			t.Fatal(err)
		}

		err = zipx.ExportFinalizeExtractedFile(
			root,
			"tmp-file",
			"target-file",
			modified,
			false,
		)
		if err != nil {
			t.Fatalf(
				"FinalizeExtractedFile() unexpected error = %v",
				err,
			)
		}

		info, err := root.Stat("target-file")
		if err != nil {
			t.Fatal(err)
		}

		if !info.ModTime().Equal(before) {
			t.Fatalf(
				"mod time = %v, want unchanged %v",
				info.ModTime(),
				before,
			)
		}
	})

	t.Run("zero modified skips chtimes", func(t *testing.T) {
		t.Parallel()

		destDir := t.TempDir()

		root, err := os.OpenRoot(destDir)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = root.Close()
		}()

		if err := root.WriteFile(
			"tmp-file",
			[]byte("data"),
			0o644,
		); err != nil {
			t.Fatal(err)
		}

		before := time.Date(
			2020, time.January, 2,
			3, 4, 5, 0,
			time.UTC,
		)

		if err := root.Chtimes("tmp-file", before, before); err != nil {
			t.Fatal(err)
		}

		err = zipx.ExportFinalizeExtractedFile(
			root,
			"tmp-file",
			"target-file",
			time.Time{},
			true,
		)
		if err != nil {
			t.Fatalf(
				"FinalizeExtractedFile() unexpected error = %v",
				err,
			)
		}

		info, err := root.Stat("target-file")
		if err != nil {
			t.Fatal(err)
		}

		if !info.ModTime().Equal(before) {
			t.Fatalf(
				"mod time = %v, want unchanged %v",
				info.ModTime(),
				before,
			)
		}
	})
}

func makeZipWithUnsupportedMethod(
	t *testing.T,
	zipPath,
	name string,
	data []byte,
) {
	t.Helper()

	makeZipWithEntryAndModified(
		t,
		zipPath,
		name,
		data,
		time.Now(),
	)

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
