package zipx

import (
	"archive/zip"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestValidate_OK(t *testing.T) {
	zp := makeZip(t, []zentry{{name: "a.txt", data: []byte("hi")}})
	if err := Validate(zp); err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}
}

func TestValidate_NotZip(t *testing.T) {
	fn := filepath.Join(t.TempDir(), "not.zip")
	if err := os.WriteFile(fn, []byte("definitely not a zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := Validate(fn)
	if err == nil {
		t.Fatalf("Validate() expected error, got nil")
	}
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("expected io.ErrUnexpectedEOF, got: %v", err)
	}
}

func TestUnzip_Simple(t *testing.T) {
	zp := makeZip(t, []zentry{
		{name: "dir", isDir: true},
		{name: "dir/a.txt", data: []byte("hello"), mode: 0o644},
	})
	dst := t.TempDir()
	if err := Unzip(zp, dst, DefaultPolicy()); err != nil {
		t.Fatalf("Unzip() error: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dst, "dir", "a.txt"))
	if err != nil {
		t.Fatalf("read extracted: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("content mismatch: %q", got)
	}
}

func TestUnzip_ZipSlipBlocked(t *testing.T) {
	zp := makeZip(t, []zentry{
		{name: "../evil.txt", data: []byte("nope")},
	})
	dst := t.TempDir()
	err := Unzip(zp, dst, DefaultPolicy())
	if err == nil {
		t.Fatalf("expected zip-slip error, got nil")
	}
	if !contains(err.Error(), "unsafe path") {
		t.Fatalf("expected unsafe path error, got: %v", err)
	}
}

func TestUnzip_MaxFiles(t *testing.T) {
	entries := []zentry{
		{name: "a.txt", data: []byte("a")},
		{name: "b.txt", data: []byte("b")},
	}
	zp := makeZip(t, entries)
	dst := t.TempDir()
	p := DefaultPolicy()
	p.MaxFiles = 1
	if err := Unzip(zp, dst, p); err == nil {
		t.Fatalf("expected too many files error, got nil")
	}
}

func TestUnzip_MaxFileBytes_Declared(t *testing.T) {
	// Any file > MaxFileBytes should be rejected via declared size check.
	big := make([]byte, 8<<10) // 8 KiB
	zp := makeZip(t, []zentry{{name: "big.bin", data: big}})
	dst := t.TempDir()
	p := DefaultPolicy()
	p.MaxFileBytes = 1024 // 1 KiB
	err := Unzip(zp, dst, p)
	if err == nil {
		t.Fatalf("expected entry too big error, got nil")
	}
	if !contains(err.Error(), "entry too big") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUnzip_SymlinkSkippedByDefault(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink mode bits are flaky on Windows")
	}
	// Store a "symlink" entry (content is the link target path)
	zp := makeZip(t, []zentry{
		{name: "link-to-a", mode: os.ModeSymlink | 0o777, data: []byte("a.txt")},
		{name: "a.txt", data: []byte("A")},
	})
	dst := t.TempDir()
	if err := Unzip(zp, dst, DefaultPolicy()); err != nil {
		t.Fatalf("Unzip() error: %v", err)
	}
	// The symlink should be skipped; a.txt should exist.
	if _, err := os.Lstat(filepath.Join(dst, "link-to-a")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected symlink to be skipped, err=%v", err)
	}
	if b, err := os.ReadFile(filepath.Join(dst, "a.txt")); err != nil || string(b) != "A" {
		t.Fatalf("expected a.txt content A, got %q err=%v", b, err)
	}
}

func TestUnzip_PreserveTimes(t *testing.T) {
	mod := time.Date(2020, 5, 6, 7, 8, 9, 0, time.UTC)
	zp := makeZip(t, []zentry{{name: "a.txt", data: []byte("x"), modified: mod}})
	dst := t.TempDir()
	p := DefaultPolicy()
	p.PreserveTimes = true
	if err := Unzip(zp, dst, p); err != nil {
		t.Fatalf("Unzip() error: %v", err)
	}
	fi, err := os.Stat(filepath.Join(dst, "a.txt"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	// Allow small drift depending on FS resolution.
	if d := fi.ModTime().Sub(mod).Abs(); d > time.Second {
		t.Fatalf("mtime not preserved (diff %v): got %v want %v", d, fi.ModTime(), mod)
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }

type zentry struct {
	name     string
	data     []byte
	mode     os.FileMode
	modified time.Time
	isDir    bool
}

func makeZip(t *testing.T, entries []zentry) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "zipx-*.zip")
	if err != nil {
		t.Fatalf("create temp zip: %v", err)
	}
	defer func() { _ = f.Close() }()

	zw := zip.NewWriter(f)
	for _, e := range entries {
		name := e.name
		if e.isDir && name[len(name)-1] != '/' {
			name += "/"
		}
		h := &zip.FileHeader{
			Name:     name,
			Modified: e.modified,
			Method:   zip.Store, // keep simple, small fixtures
		}
		if e.isDir {
			h.SetMode(os.ModeDir | 0o755)
		} else if e.mode != 0 {
			h.SetMode(e.mode)
		} else {
			h.SetMode(0o644)
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatalf("create header: %v", err)
		}
		if !e.isDir && len(e.data) > 0 {
			if _, err := w.Write(e.data); err != nil {
				t.Fatalf("write entry: %v", err)
			}
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return f.Name()
}
