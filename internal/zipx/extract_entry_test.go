package zipx_test

import (
	"archive/zip"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bodrovis/lokex/v2/internal/zipx"
)

func TestExtractEntry(t *testing.T) {
	t.Parallel()

	t.Run("special file mode is skipped", func(t *testing.T) {
		t.Parallel()

		destDir := t.TempDir()

		root, err := os.OpenRoot(destDir)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = root.Close()
		}()

		f := &zip.File{
			FileHeader: zip.FileHeader{
				Name:   "pipe",
				Method: zip.Store,
			},
		}
		f.SetMode(os.ModeNamedPipe)

		n, err := zipx.ExportExtractEntry(
			f,
			root,
			zipx.DefaultPolicy(),
		)
		if err != nil {
			t.Fatalf("ExtractEntry() unexpected error = %v", err)
		}

		if n != 0 {
			t.Fatalf("n = %d, want 0", n)
		}

		if _, err := root.Lstat("pipe"); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("special entry was created: Lstat() error = %v", err)
		}
	})
}

func TestPrepareEntryTarget(t *testing.T) {
	t.Parallel()

	t.Run("empty normalized path is skipped", func(t *testing.T) {
		t.Parallel()

		f := &zip.File{
			FileHeader: zip.FileHeader{
				Name: ".",
			},
		}

		rel, mode, skip, err := zipx.ExportPrepareEntryTarget(
			f,
			zipx.DefaultPolicy(),
		)
		if err != nil {
			t.Fatalf("PrepareEntryTarget() unexpected error = %v", err)
		}

		if rel != "" {
			t.Fatalf("rel = %q, want empty", rel)
		}

		if mode != 0 {
			t.Fatalf("mode = %v, want 0", mode)
		}

		if !skip {
			t.Fatal("skip = false, want true")
		}
	})

	t.Run("unsafe path is rejected", func(t *testing.T) {
		t.Parallel()

		f := &zip.File{
			FileHeader: zip.FileHeader{
				Name: "../evil.txt",
			},
		}

		_, _, skip, err := zipx.ExportPrepareEntryTarget(
			f,
			zipx.DefaultPolicy(),
		)

		if err == nil {
			t.Fatal("PrepareEntryTarget() error = nil, want non-nil")
		}

		if skip {
			t.Fatal("skip = true, want false")
		}
	})

	t.Run("file exceeding header limit is rejected", func(t *testing.T) {
		t.Parallel()

		f := &zip.File{
			FileHeader: zip.FileHeader{
				Name:               "large.bin",
				UncompressedSize64: 101,
			},
		}

		p := zipx.DefaultPolicy()
		p.MaxFileBytes = 100

		_, _, skip, err := zipx.ExportPrepareEntryTarget(f, p)

		if err == nil {
			t.Fatal("PrepareEntryTarget() error = nil, want non-nil")
		}

		if skip {
			t.Fatal("skip = true, want false")
		}

		if !strings.Contains(err.Error(), "zip entry too big by header") {
			t.Fatalf("error = %q, want size limit error", err.Error())
		}
	})
}

func TestExtractDirEntry(t *testing.T) {
	t.Parallel()

	destDir := t.TempDir()

	// Make "blocked" a regular file, so creating blocked/dir must fail.
	if err := os.WriteFile(
		filepath.Join(destDir, "blocked"),
		[]byte("x"),
		0o644,
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

	f := &zip.File{
		FileHeader: zip.FileHeader{
			Name: "blocked/dir/",
		},
	}

	err = zipx.ExportExtractDirEntry(
		f,
		root,
		"blocked/dir",
		zipx.DefaultPolicy(),
	)
	if err == nil {
		t.Fatal("ExtractDirEntry() error = nil, want non-nil")
	}
}

func makeZipWithEntry(t *testing.T, zipPath, name string, data []byte) {
	t.Helper()

	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = f.Close()
	}()

	zw := zip.NewWriter(f)
	w, err := zw.Create(name)
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
