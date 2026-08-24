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
	t.Parallel()

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
			t.Parallel()

			err := zipx.ExportValidateSymlinkTargetString(
				tt.entry,
				tt.target,
			)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("ValidateSymlinkTargetString() error = nil, want non-nil")
				}

				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf(
						"error = %q, want to contain %q",
						err.Error(),
						tt.wantErr,
					)
				}

				return
			}

			if err != nil {
				t.Fatalf(
					"ValidateSymlinkTargetString() unexpected error = %v",
					err,
				)
			}
		})
	}
}

func TestReadSymlinkTarget(t *testing.T) {
	// Do not run these subtests in parallel: one of them replaces
	// the package-level readAllFn test seam.

	t.Run("open error", func(t *testing.T) {
		tmpDir := t.TempDir()
		zipPath := filepath.Join(tmpDir, "bad.zip")

		makeZipWithUnsupportedMethod(
			t,
			zipPath,
			"link",
			[]byte("target.txt"),
		)

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
		restore := zipx.ExportSetReadAllForTest(
			func(io.Reader) ([]byte, error) {
				return nil, errors.New("read boom")
			},
		)
		defer restore()

		tmpDir := t.TempDir()
		zipPath := filepath.Join(tmpDir, "a.zip")

		makeZipWithEntry(
			t,
			zipPath,
			"link",
			[]byte("target.txt"),
		)

		zr, f := openFirstZipFile(t, zipPath)
		defer func() {
			_ = zr.Close()
		}()

		_, err := zipx.ExportReadSymlinkTarget(f)
		if err == nil {
			t.Fatal("ReadSymlinkTarget() error = nil, want non-nil")
		}

		if err.Error() != "read symlink target: read boom" {
			t.Fatalf(
				"error = %q, want %q",
				err.Error(),
				"read symlink target: read boom",
			)
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
			t.Fatalf(
				"error = %q, want %q",
				err.Error(),
				"symlink target too large",
			)
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
			t.Fatalf(
				"ReadSymlinkTarget() unexpected error = %v",
				err,
			)
		}

		if len(got) != 1<<20 {
			t.Fatalf(
				"len(target) = %d, want %d",
				len(got),
				1<<20,
			)
		}
	})

	t.Run("trims whitespace", func(t *testing.T) {
		tmpDir := t.TempDir()
		zipPath := filepath.Join(tmpDir, "a.zip")

		makeZipWithEntry(
			t,
			zipPath,
			"link",
			[]byte(" \n\t target.txt \r\n"),
		)

		zr, f := openFirstZipFile(t, zipPath)
		defer func() {
			_ = zr.Close()
		}()

		got, err := zipx.ExportReadSymlinkTarget(f)
		if err != nil {
			t.Fatalf(
				"ReadSymlinkTarget() unexpected error = %v",
				err,
			)
		}

		if got != "target.txt" {
			t.Fatalf(
				"target = %q, want %q",
				got,
				"target.txt",
			)
		}
	})

	t.Run("success", func(t *testing.T) {
		tmpDir := t.TempDir()
		zipPath := filepath.Join(tmpDir, "a.zip")

		makeZipWithEntry(
			t,
			zipPath,
			"link",
			[]byte("dir/file.txt"),
		)

		zr, f := openFirstZipFile(t, zipPath)
		defer func() {
			_ = zr.Close()
		}()

		got, err := zipx.ExportReadSymlinkTarget(f)
		if err != nil {
			t.Fatalf(
				"ReadSymlinkTarget() unexpected error = %v",
				err,
			)
		}

		if got != "dir/file.txt" {
			t.Fatalf(
				"target = %q, want %q",
				got,
				"dir/file.txt",
			)
		}
	})
}

func TestValidateSymlinkPlacement(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		entryName  string
		rel        string
		linkTarget string
		wantErr    bool
	}{
		{
			name:       "same directory target",
			entryName:  "dir/link",
			rel:        filepath.Join("dir", "link"),
			linkTarget: "target.txt",
		},
		{
			name:       "nested target",
			entryName:  "dir/link",
			rel:        filepath.Join("dir", "link"),
			linkTarget: "nested/target.txt",
		},
		{
			name:       "parent traversal staying inside root",
			entryName:  "dir/link",
			rel:        filepath.Join("dir", "link"),
			linkTarget: "../target.txt",
		},
		{
			name:       "target escapes root",
			entryName:  "dir/link",
			rel:        filepath.Join("dir", "link"),
			linkTarget: "../../outside.txt",
			wantErr:    true,
		},
		{
			name:       "root-level target escapes root",
			entryName:  "link",
			rel:        "link",
			linkTarget: "../outside.txt",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := zipx.ExportValidateSymlinkPlacement(
				tt.entryName,
				tt.rel,
				tt.linkTarget,
			)

			if tt.wantErr {
				if err == nil {
					t.Fatal("ValidateSymlinkPlacement() error = nil, want non-nil")
				}

				if !strings.Contains(
					err.Error(),
					"symlink target escapes extraction root",
				) {
					t.Fatalf(
						"error = %q, want escape error",
						err.Error(),
					)
				}

				return
			}

			if err != nil {
				t.Fatalf(
					"ValidateSymlinkPlacement() unexpected error = %v",
					err,
				)
			}
		})
	}
}

func TestExtractSymlinkEntry(t *testing.T) {
	// Do not run these subtests in parallel: some replace the
	// package-level symlinkFn test seam.

	t.Run("symlinks disabled", func(t *testing.T) {
		tmpDir := t.TempDir()

		root, err := os.OpenRoot(tmpDir)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = root.Close()
		}()

		zipPath := filepath.Join(tmpDir, "a.zip")
		makeZipWithEntry(
			t,
			zipPath,
			"link",
			[]byte("target.txt"),
		)

		zr, f := openFirstZipFile(t, zipPath)
		defer func() {
			_ = zr.Close()
		}()

		called := false
		restore := zipx.ExportSetSymlinkForTest(
			func(*os.Root, string, string) error {
				called = true
				return nil
			},
		)
		defer restore()

		err = zipx.ExportExtractSymlinkEntry(
			f,
			root,
			"link",
			zipx.Policy{AllowSymlinks: false},
		)
		if err != nil {
			t.Fatalf(
				"ExtractSymlinkEntry() unexpected error = %v",
				err,
			)
		}

		if called {
			t.Fatal("symlink creation called with AllowSymlinks=false")
		}
	})

	t.Run("read symlink target error", func(t *testing.T) {
		tmpDir := t.TempDir()

		root, err := os.OpenRoot(tmpDir)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = root.Close()
		}()

		zipPath := filepath.Join(tmpDir, "bad.zip")
		makeZipWithUnsupportedMethod(
			t,
			zipPath,
			"link",
			[]byte("target.txt"),
		)

		zr, f := openFirstZipFile(t, zipPath)
		defer func() {
			_ = zr.Close()
		}()

		err = zipx.ExportExtractSymlinkEntry(
			f,
			root,
			"link",
			zipx.Policy{AllowSymlinks: true},
		)
		if err == nil {
			t.Fatal("ExtractSymlinkEntry() error = nil, want non-nil")
		}
	})

	t.Run("invalid symlink target string", func(t *testing.T) {
		tmpDir := t.TempDir()

		root, err := os.OpenRoot(tmpDir)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = root.Close()
		}()

		var target []byte
		if filepath.Separator == '\\' {
			target = []byte(`C:\outside.txt`)
		} else {
			target = []byte("/outside.txt")
		}

		zipPath := filepath.Join(tmpDir, "a.zip")
		makeZipWithEntry(t, zipPath, "link", target)

		zr, f := openFirstZipFile(t, zipPath)
		defer func() {
			_ = zr.Close()
		}()

		err = zipx.ExportExtractSymlinkEntry(
			f,
			root,
			"link",
			zipx.Policy{AllowSymlinks: true},
		)
		if err == nil {
			t.Fatal("ExtractSymlinkEntry() error = nil, want non-nil")
		}

		if !strings.Contains(
			err.Error(),
			"absolute symlink target not allowed",
		) {
			t.Fatalf(
				"error = %q, want target validation error",
				err.Error(),
			)
		}
	})

	t.Run("placement validation error", func(t *testing.T) {
		tmpDir := t.TempDir()

		root, err := os.OpenRoot(tmpDir)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = root.Close()
		}()

		zipPath := filepath.Join(tmpDir, "a.zip")
		makeZipWithEntry(
			t,
			zipPath,
			"link",
			[]byte("../../outside.txt"),
		)

		zr, f := openFirstZipFile(t, zipPath)
		defer func() {
			_ = zr.Close()
		}()

		err = zipx.ExportExtractSymlinkEntry(
			f,
			root,
			filepath.Join("dir", "link"),
			zipx.Policy{AllowSymlinks: true},
		)
		if err == nil {
			t.Fatal("ExtractSymlinkEntry() error = nil, want non-nil")
		}

		if !strings.Contains(
			err.Error(),
			"symlink target escapes extraction root",
		) {
			t.Fatalf(
				"error = %q, want placement error",
				err.Error(),
			)
		}
	})

	t.Run("symlink create error", func(t *testing.T) {
		tmpDir := t.TempDir()

		root, err := os.OpenRoot(tmpDir)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = root.Close()
		}()

		zipPath := filepath.Join(tmpDir, "a.zip")
		makeZipWithEntry(
			t,
			zipPath,
			"link",
			[]byte("target.txt"),
		)

		zr, f := openFirstZipFile(t, zipPath)
		defer func() {
			_ = zr.Close()
		}()

		restore := zipx.ExportSetSymlinkForTest(
			func(*os.Root, string, string) error {
				return errors.New("symlink boom")
			},
		)
		defer restore()

		err = zipx.ExportExtractSymlinkEntry(
			f,
			root,
			"link",
			zipx.Policy{AllowSymlinks: true},
		)
		if err == nil {
			t.Fatal("ExtractSymlinkEntry() error = nil, want non-nil")
		}

		if err.Error() != "create symlink: symlink boom" {
			t.Fatalf(
				"error = %q, want %q",
				err.Error(),
				"create symlink: symlink boom",
			)
		}
	})

	t.Run("success removes existing entry and creates symlink", func(t *testing.T) {
		tmpDir := t.TempDir()

		root, err := os.OpenRoot(tmpDir)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = root.Close()
		}()

		if err := root.WriteFile(
			"link",
			[]byte("old"),
			0o644,
		); err != nil {
			t.Fatal(err)
		}

		zipPath := filepath.Join(tmpDir, "a.zip")
		makeZipWithEntry(
			t,
			zipPath,
			"link",
			[]byte(" target.txt \n"),
		)

		zr, f := openFirstZipFile(t, zipPath)
		defer func() {
			_ = zr.Close()
		}()

		var gotTarget string
		var gotName string

		restore := zipx.ExportSetSymlinkForTest(
			func(_ *os.Root, target, name string) error {
				gotTarget = target
				gotName = name
				return nil
			},
		)
		defer restore()

		err = zipx.ExportExtractSymlinkEntry(
			f,
			root,
			"link",
			zipx.Policy{AllowSymlinks: true},
		)
		if err != nil {
			t.Fatalf(
				"ExtractSymlinkEntry() unexpected error = %v",
				err,
			)
		}

		if _, err := root.Lstat("link"); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf(
				"existing entry was not removed: Lstat() error = %v",
				err,
			)
		}

		if gotTarget != "target.txt" {
			t.Fatalf(
				"symlink target = %q, want %q",
				gotTarget,
				"target.txt",
			)
		}

		if gotName != "link" {
			t.Fatalf(
				"symlink name = %q, want %q",
				gotName,
				"link",
			)
		}
	})
}

func openFirstZipFile(
	t *testing.T,
	zipPath string,
) (*zip.ReadCloser, *zip.File) {
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
