package zipx_test

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bodrovis/lokex/v2/internal/zipx"
)

func TestValidateSymlinkTargetString(t *testing.T) {
	tests := []struct {
		name    string
		entry   string
		target  string
		wantErr string
	}{
		{
			name:    "empty target",
			entry:   "link",
			target:  "",
			wantErr: `empty symlink target: "link"`,
		},
		{
			name:   "relative target ok",
			entry:  "link",
			target: "dir/file.txt",
		},
		{
			name:   "dot relative target ok",
			entry:  "link",
			target: "./file.txt",
		},
	}

	if filepath.Separator == '\\' {
		tests = append(tests, struct {
			name    string
			entry   string
			target  string
			wantErr string
		}{
			name:    "absolute windows target rejected",
			entry:   "link",
			target:  `C:\tmp\file.txt`,
			wantErr: `"C:\\tmp\\file.txt"`,
		})
	} else {
		tests = append(tests, struct {
			name    string
			entry   string
			target  string
			wantErr string
		}{
			name:    "absolute unix target rejected",
			entry:   "link",
			target:  "/tmp/file.txt",
			wantErr: `absolute symlink target not allowed: "link" -> "/tmp/file.txt"`,
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := zipx.ExportValidateSymlinkTargetString(tt.entry, tt.target)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("ValidateSymlinkTargetString() error = nil, want non-nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %q, want to contain %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateSymlinkTargetString() unexpected error = %v", err)
			}
		})
	}
}

func TestReadSymlinkTarget(t *testing.T) {
	t.Run("open error", func(t *testing.T) {
		tmpDir := t.TempDir()
		zipPath := filepath.Join(tmpDir, "bad.zip")
		makeZipWithUnsupportedMethod(t, zipPath, "link", []byte("target.txt"))

		zr, f := openFirstZipFile(t, zipPath)
		defer func() {
			_ = zr.Close()
		}()

		_, err := zipx.ExportReadSymlinkTarget(f)
		if err == nil {
			t.Fatal("ReadSymlinkTarget() error = nil, want non-nil")
		}
	})

	t.Run("read error", func(t *testing.T) {
		restore := zipx.ExportSetReadAllForTest(func(io.Reader) ([]byte, error) {
			return nil, errors.New("read boom")
		})
		defer restore()

		tmpDir := t.TempDir()
		zipPath := filepath.Join(tmpDir, "a.zip")
		makeZipWithEntry(t, zipPath, "link", []byte("target.txt"))

		zr, f := openFirstZipFile(t, zipPath)
		defer func() {
			_ = zr.Close()
		}()

		_, err := zipx.ExportReadSymlinkTarget(f)
		if err == nil {
			t.Fatal("ReadSymlinkTarget() error = nil, want non-nil")
		}
		if err.Error() != "read symlink target: read boom" {
			t.Fatalf("error = %q, want %q", err.Error(), "read symlink target: read boom")
		}
	})

	t.Run("target too large", func(t *testing.T) {
		tmpDir := t.TempDir()
		zipPath := filepath.Join(tmpDir, "a.zip")
		data := bytes.Repeat([]byte("a"), (1<<20)+1)
		makeZipWithEntry(t, zipPath, "link", data)

		zr, f := openFirstZipFile(t, zipPath)
		defer func() {
			_ = zr.Close()
		}()

		_, err := zipx.ExportReadSymlinkTarget(f)
		if err == nil {
			t.Fatal("ReadSymlinkTarget() error = nil, want non-nil")
		}
		if err.Error() != "symlink target too large" {
			t.Fatalf("error = %q, want %q", err.Error(), "symlink target too large")
		}
	})

	t.Run("target at exact limit is allowed", func(t *testing.T) {
		tmpDir := t.TempDir()
		zipPath := filepath.Join(tmpDir, "a.zip")
		data := bytes.Repeat([]byte("a"), 1<<20)
		makeZipWithEntry(t, zipPath, "link", data)

		zr, f := openFirstZipFile(t, zipPath)
		defer func() {
			_ = zr.Close()
		}()

		got, err := zipx.ExportReadSymlinkTarget(f)
		if err != nil {
			t.Fatalf("ReadSymlinkTarget() unexpected error = %v", err)
		}
		if len(got) != 1<<20 {
			t.Fatalf("len(target) = %d, want %d", len(got), 1<<20)
		}
	})

	t.Run("trims whitespace", func(t *testing.T) {
		tmpDir := t.TempDir()
		zipPath := filepath.Join(tmpDir, "a.zip")
		makeZipWithEntry(t, zipPath, "link", []byte(" \n\t target.txt \r\n"))

		zr, f := openFirstZipFile(t, zipPath)
		defer func() {
			_ = zr.Close()
		}()

		got, err := zipx.ExportReadSymlinkTarget(f)
		if err != nil {
			t.Fatalf("ReadSymlinkTarget() unexpected error = %v", err)
		}
		if got != "target.txt" {
			t.Fatalf("target = %q, want %q", got, "target.txt")
		}
	})

	t.Run("success", func(t *testing.T) {
		tmpDir := t.TempDir()
		zipPath := filepath.Join(tmpDir, "a.zip")
		makeZipWithEntry(t, zipPath, "link", []byte("dir/file.txt"))

		zr, f := openFirstZipFile(t, zipPath)
		defer func() {
			_ = zr.Close()
		}()

		got, err := zipx.ExportReadSymlinkTarget(f)
		if err != nil {
			t.Fatalf("ReadSymlinkTarget() unexpected error = %v", err)
		}
		if got != "dir/file.txt" {
			t.Fatalf("target = %q, want %q", got, "dir/file.txt")
		}
	})
}

func TestValidateSymlinkPlacement(t *testing.T) {
	t.Run("parent resolve non-not-exist error", func(t *testing.T) {
		restoreEval := zipx.ExportSetEvalSymlinksPathForTest(func(string) (string, error) {
			return "", errors.New("eval boom")
		})
		defer restoreEval()

		err := zipx.ExportValidateSymlinkPlacement(
			"link",
			filepath.Join("root", "dir", "link"),
			"root",
			"target.txt",
		)
		if err == nil {
			t.Fatal("ValidateSymlinkPlacement() error = nil, want non-nil")
		}
		if err.Error() != "symlink parent resolve error: eval boom" {
			t.Fatalf("error = %q, want %q", err.Error(), "symlink parent resolve error: eval boom")
		}
	})

	t.Run("parent not exist falls back to dir", func(t *testing.T) {
		restoreEval := zipx.ExportSetEvalSymlinksPathForTest(func(string) (string, error) {
			return "", os.ErrNotExist
		})
		defer restoreEval()

		restoreWithin := zipx.ExportSetIsPathWithinBaseForTest(func(base, p string) bool {
			return true
		})
		defer restoreWithin()

		err := zipx.ExportValidateSymlinkPlacement(
			"link",
			filepath.Join("root", "dir", "link"),
			"root",
			"target.txt",
		)
		if err != nil {
			t.Fatalf("ValidateSymlinkPlacement() unexpected error = %v", err)
		}
	})

	t.Run("destination escapes extraction root", func(t *testing.T) {
		restoreEval := zipx.ExportSetEvalSymlinksPathForTest(func(string) (string, error) {
			return filepath.Join("root", "dir"), nil
		})
		defer restoreEval()

		call := 0
		restoreWithin := zipx.ExportSetIsPathWithinBaseForTest(func(base, p string) bool {
			call++
			return call != 1
		})
		defer restoreWithin()

		err := zipx.ExportValidateSymlinkPlacement(
			"link",
			filepath.Join("root", "dir", "link"),
			"root",
			"target.txt",
		)
		if err == nil {
			t.Fatal("ValidateSymlinkPlacement() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "symlink destination escapes extraction root") {
			t.Fatalf("error = %q, want destination escape error", err.Error())
		}
	})

	t.Run("target escapes extraction root", func(t *testing.T) {
		restoreEval := zipx.ExportSetEvalSymlinksPathForTest(func(string) (string, error) {
			return filepath.Join("root", "dir"), nil
		})
		defer restoreEval()

		call := 0
		restoreWithin := zipx.ExportSetIsPathWithinBaseForTest(func(base, p string) bool {
			call++
			return call == 1
		})
		defer restoreWithin()

		err := zipx.ExportValidateSymlinkPlacement(
			"entry-link",
			filepath.Join("root", "dir", "link"),
			"root",
			"../outside.txt",
		)
		if err == nil {
			t.Fatal("ValidateSymlinkPlacement() error = nil, want non-nil")
		}
		if err.Error() != `symlink target escapes extraction root: "entry-link" -> "../outside.txt"` {
			t.Fatalf("error = %q, want %q", err.Error(), `symlink target escapes extraction root: "entry-link" -> "../outside.txt"`)
		}
	})

	t.Run("success", func(t *testing.T) {
		restoreEval := zipx.ExportSetEvalSymlinksPathForTest(func(string) (string, error) {
			return filepath.Join("root", "dir"), nil
		})
		defer restoreEval()

		restoreWithin := zipx.ExportSetIsPathWithinBaseForTest(func(base, p string) bool {
			return true
		})
		defer restoreWithin()

		err := zipx.ExportValidateSymlinkPlacement(
			"link",
			filepath.Join("root", "dir", "link"),
			"root",
			"target.txt",
		)
		if err != nil {
			t.Fatalf("ValidateSymlinkPlacement() unexpected error = %v", err)
		}
	})
}

func TestExtractSymlinkEntry(t *testing.T) {
	t.Run("symlinks disabled", func(t *testing.T) {
		tmpDir := t.TempDir()
		zipPath := filepath.Join(tmpDir, "a.zip")
		makeZipWithEntry(t, zipPath, "link", []byte("target.txt"))

		zr, f := openFirstZipFile(t, zipPath)
		defer func() {
			_ = zr.Close()
		}()

		called := false
		restoreSymlink := zipx.ExportSetSymlinkForTest(func(string, string) error {
			called = true
			return nil
		})
		defer restoreSymlink()

		err := zipx.ExportExtractSymlinkEntry(
			f,
			filepath.Join(tmpDir, "link"),
			tmpDir,
			zipx.Policy{AllowSymlinks: false},
		)
		if err != nil {
			t.Fatalf("ExtractSymlinkEntry() unexpected error = %v", err)
		}
		if called {
			t.Fatal("symlink creation called with AllowSymlinks=false")
		}
	})

	t.Run("read symlink target error", func(t *testing.T) {
		tmpDir := t.TempDir()
		zipPath := filepath.Join(tmpDir, "bad.zip")
		makeZipWithUnsupportedMethod(t, zipPath, "link", []byte("target.txt"))

		zr, f := openFirstZipFile(t, zipPath)
		defer func() {
			_ = zr.Close()
		}()

		err := zipx.ExportExtractSymlinkEntry(
			f,
			filepath.Join(tmpDir, "link"),
			tmpDir,
			zipx.Policy{AllowSymlinks: true},
		)
		if err == nil {
			t.Fatal("ExtractSymlinkEntry() error = nil, want non-nil")
		}
	})

	t.Run("invalid symlink target string", func(t *testing.T) {
		tmpDir := t.TempDir()
		zipPath := filepath.Join(tmpDir, "a.zip")

		var target []byte
		if filepath.Separator == '\\' {
			target = []byte(`C:\outside.txt`)
		} else {
			target = []byte(`/outside.txt`)
		}
		makeZipWithEntry(t, zipPath, "link", target)

		zr, f := openFirstZipFile(t, zipPath)
		defer func() {
			_ = zr.Close()
		}()

		err := zipx.ExportExtractSymlinkEntry(
			f,
			filepath.Join(tmpDir, "link"),
			tmpDir,
			zipx.Policy{AllowSymlinks: true},
		)
		if err == nil {
			t.Fatal("ExtractSymlinkEntry() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "absolute symlink target not allowed") {
			t.Fatalf("error = %q, want target validation error", err.Error())
		}
	})

	t.Run("placement validation error", func(t *testing.T) {
		restoreEval := zipx.ExportSetEvalSymlinksPathForTest(func(string) (string, error) {
			return filepath.Join("root", "dir"), nil
		})
		defer restoreEval()

		call := 0
		restoreWithin := zipx.ExportSetIsPathWithinBaseForTest(func(base, p string) bool {
			call++
			return call != 1
		})
		defer restoreWithin()

		tmpDir := t.TempDir()
		zipPath := filepath.Join(tmpDir, "a.zip")
		makeZipWithEntry(t, zipPath, "link", []byte("target.txt"))

		zr, f := openFirstZipFile(t, zipPath)
		defer func() {
			_ = zr.Close()
		}()

		err := zipx.ExportExtractSymlinkEntry(
			f,
			filepath.Join(tmpDir, "link"),
			tmpDir,
			zipx.Policy{AllowSymlinks: true},
		)
		if err == nil {
			t.Fatal("ExtractSymlinkEntry() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "symlink destination escapes extraction root") {
			t.Fatalf("error = %q, want placement error", err.Error())
		}
	})

	t.Run("symlink create error", func(t *testing.T) {
		restoreEval := zipx.ExportSetEvalSymlinksPathForTest(func(string) (string, error) {
			return filepath.Dir(filepath.Join("root", "dir", "link")), nil
		})
		defer restoreEval()

		restoreWithin := zipx.ExportSetIsPathWithinBaseForTest(func(base, p string) bool {
			return true
		})
		defer restoreWithin()

		restoreSymlink := zipx.ExportSetSymlinkForTest(func(string, string) error {
			return errors.New("symlink boom")
		})
		defer restoreSymlink()

		tmpDir := t.TempDir()
		zipPath := filepath.Join(tmpDir, "a.zip")
		makeZipWithEntry(t, zipPath, "link", []byte("target.txt"))

		zr, f := openFirstZipFile(t, zipPath)
		defer func() {
			_ = zr.Close()
		}()

		err := zipx.ExportExtractSymlinkEntry(
			f,
			filepath.Join(tmpDir, "link"),
			tmpDir,
			zipx.Policy{AllowSymlinks: true},
		)
		if err == nil {
			t.Fatal("ExtractSymlinkEntry() error = nil, want non-nil")
		}
		if err.Error() != "create symlink: symlink boom" {
			t.Fatalf("error = %q, want %q", err.Error(), "create symlink: symlink boom")
		}
	})

	t.Run("success", func(t *testing.T) {
		removed := ""
		restoreRemove := zipx.ExportSetRemovePathForTest(func(name string) error {
			removed = name
			return nil
		})
		defer restoreRemove()

		restoreEval := zipx.ExportSetEvalSymlinksPathForTest(func(string) (string, error) {
			return filepath.Join("root", "dir"), nil
		})
		defer restoreEval()

		restoreWithin := zipx.ExportSetIsPathWithinBaseForTest(func(base, p string) bool {
			return true
		})
		defer restoreWithin()

		var gotTarget, gotPath string
		restoreSymlink := zipx.ExportSetSymlinkForTest(func(target, path string) error {
			gotTarget = target
			gotPath = path
			return nil
		})
		defer restoreSymlink()

		tmpDir := t.TempDir()
		zipPath := filepath.Join(tmpDir, "a.zip")
		makeZipWithEntry(t, zipPath, "link", []byte(" target.txt \n"))

		zr, f := openFirstZipFile(t, zipPath)
		defer func() {
			_ = zr.Close()
		}()

		targetAbs := filepath.Join(tmpDir, "link")
		err := zipx.ExportExtractSymlinkEntry(
			f,
			targetAbs,
			tmpDir,
			zipx.Policy{AllowSymlinks: true},
		)
		if err != nil {
			t.Fatalf("ExtractSymlinkEntry() unexpected error = %v", err)
		}

		if removed != targetAbs {
			t.Fatalf("removed path = %q, want %q", removed, targetAbs)
		}
		if gotTarget != "target.txt" {
			t.Fatalf("symlink target = %q, want %q", gotTarget, "target.txt")
		}
		if gotPath != targetAbs {
			t.Fatalf("symlink path = %q, want %q", gotPath, targetAbs)
		}
	})
}

func openFirstZipFile(t *testing.T, zipPath string) (*zip.ReadCloser, *zip.File) {
	t.Helper()

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(zr.File) == 0 {
		_ = zr.Close()
		t.Fatal("zip has no entries")
	}
	return zr, zr.File[0]
}
