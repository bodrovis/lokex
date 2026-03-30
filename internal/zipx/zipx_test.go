package zipx_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/bodrovis/lokex/v2/internal/zipx"
)

func TestUnzip_Simple(t *testing.T) {
	zp := makeZip(t, []zentry{
		{name: "dir", isDir: true},
		{name: "dir/a.txt", data: []byte("hello"), mode: 0o644},
	})
	dst := t.TempDir()
	if err := zipx.Unzip(zp, dst, zipx.DefaultPolicy()); err != nil {
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
	err := zipx.Unzip(zp, dst, zipx.DefaultPolicy())
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
	p := zipx.DefaultPolicy()
	p.MaxFiles = 1
	if err := zipx.Unzip(zp, dst, p); err == nil {
		t.Fatalf("expected too many files error, got nil")
	}
}

func TestUnzip_MaxFileBytes_Declared(t *testing.T) {
	// Any file > MaxFileBytes should be rejected via declared size check.
	big := make([]byte, 8<<10) // 8 KiB
	zp := makeZip(t, []zentry{{name: "big.bin", data: big}})
	dst := t.TempDir()
	p := zipx.DefaultPolicy()
	p.MaxFileBytes = 1024 // 1 KiB
	err := zipx.Unzip(zp, dst, p)
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
	if err := zipx.Unzip(zp, dst, zipx.DefaultPolicy()); err != nil {
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
	p := zipx.DefaultPolicy()
	p.PreserveTimes = true
	if err := zipx.Unzip(zp, dst, p); err != nil {
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

func TestUnzip_NormalizesLeadingSlashAndBackslashes(t *testing.T) {
	zp := makeZip(t, []zentry{
		{name: "/rooted/ok.txt", data: []byte("1")},
		{name: `win\path\ok2.txt`, data: []byte("2")},
	})
	dst := t.TempDir()

	if err := zipx.Unzip(zp, dst, zipx.DefaultPolicy()); err != nil {
		t.Fatalf("Unzip() error: %v", err)
	}

	if b, err := os.ReadFile(filepath.Join(dst, "rooted", "ok.txt")); err != nil || string(b) != "1" {
		t.Fatalf("rooted/ok.txt missing/wrong: %v %q", err, b)
	}
	if b, err := os.ReadFile(filepath.Join(dst, "win", "path", "ok2.txt")); err != nil || string(b) != "2" {
		t.Fatalf("win/path/ok2.txt missing/wrong: %v %q", err, b)
	}
}

func TestUnzip_DotDotSegmentBlocked(t *testing.T) {
	zp := makeZip(t, []zentry{
		{name: "a/../../evil.txt", data: []byte("nope")},
	})
	dst := t.TempDir()

	err := zipx.Unzip(zp, dst, zipx.DefaultPolicy())
	if err == nil {
		t.Fatalf("expected traversal error, got nil")
	}
	if !strings.Contains(err.Error(), ".. segment") && !strings.Contains(err.Error(), "unsafe") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUnzip_MaxTotalBytes(t *testing.T) {
	zp := makeZip(t, []zentry{
		{name: "a.bin", data: bytes.Repeat([]byte("a"), 80<<10)},
		{name: "b.bin", data: bytes.Repeat([]byte("b"), 80<<10)},
	})
	dst := t.TempDir()

	p := zipx.DefaultPolicy()
	p.MaxTotalBytes = 100 << 10 // 100KiB

	err := zipx.Unzip(zp, dst, p)
	if err == nil {
		t.Fatalf("expected total bytes cap error, got nil")
	}
	if !strings.Contains(err.Error(), "zip too large uncompressed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUnzip_DestDirIsFile(t *testing.T) {
	zp := makeZip(t, []zentry{
		{name: "a.txt", data: []byte("hello")},
	})

	dstFile := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(dstFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := zipx.Unzip(zp, dstFile, zipx.DefaultPolicy())
	if err == nil {
		t.Fatalf("expected error when destDir is a file, got nil")
	}
}

func TestUnzip_OverwritesExistingFile(t *testing.T) {
	zp := makeZip(t, []zentry{
		{name: "a.txt", data: []byte("new")},
	})

	dst := t.TempDir()
	target := filepath.Join(dst, "a.txt")
	if err := os.WriteFile(target, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := zipx.Unzip(zp, dst, zipx.DefaultPolicy()); err != nil {
		t.Fatalf("Unzip() error: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read extracted: %v", err)
	}
	if string(got) != "new" {
		t.Fatalf("content mismatch: got %q want %q", got, "new")
	}
}

func TestUnzip_ParentPathIsFile(t *testing.T) {
	zp := makeZip(t, []zentry{
		{name: "a/b.txt", data: []byte("x")},
	})

	dst := t.TempDir()
	parentAsFile := filepath.Join(dst, "a")
	if err := os.WriteFile(parentAsFile, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := zipx.Unzip(zp, dst, zipx.DefaultPolicy())
	if err == nil {
		t.Fatalf("expected error when parent path is a file, got nil")
	}
}

func TestUnzip_UnsafeParentSymlinkBlocked(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink behavior is flaky on Windows")
	}

	zp := makeZip(t, []zentry{
		{name: "link/evil.txt", data: []byte("nope")},
	})

	base := t.TempDir()
	dst := filepath.Join(base, "dst")
	outside := filepath.Join(base, "outside")

	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(outside, filepath.Join(dst, "link")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	err := zipx.Unzip(zp, dst, zipx.DefaultPolicy())
	if err == nil {
		t.Fatalf("expected unsafe symlink error, got nil")
	}
	if !strings.Contains(err.Error(), "unsafe symlink in parents") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUnzip_AllowSymlinks_CreatesRelativeSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink behavior is flaky on Windows")
	}

	zp := makeZip(t, []zentry{
		{name: "a.txt", data: []byte("A")},
		{name: "link-to-a", mode: os.ModeSymlink | 0o777, data: []byte("a.txt")},
	})

	dst := t.TempDir()
	p := zipx.DefaultPolicy()
	p.AllowSymlinks = true

	if err := zipx.Unzip(zp, dst, p); err != nil {
		t.Fatalf("Unzip() error: %v", err)
	}

	fi, err := os.Lstat(filepath.Join(dst, "link-to-a"))
	if err != nil {
		t.Fatalf("lstat symlink: %v", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected symlink, mode=%v", fi.Mode())
	}

	target, err := os.Readlink(filepath.Join(dst, "link-to-a"))
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	if target != "a.txt" {
		t.Fatalf("symlink target = %q, want %q", target, "a.txt")
	}
}

func TestUnzip_AllowSymlinks_RejectsAbsoluteTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("absolute symlink target case is platform-specific on Windows")
	}

	zp := makeZip(t, []zentry{
		{name: "bad-link", mode: os.ModeSymlink | 0o777, data: []byte("/etc/passwd")},
	})

	dst := t.TempDir()
	p := zipx.DefaultPolicy()
	p.AllowSymlinks = true

	err := zipx.Unzip(zp, dst, p)
	if err == nil {
		t.Fatalf("expected absolute symlink target error, got nil")
	}
	if !strings.Contains(err.Error(), "absolute symlink target not allowed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUnzip_AllowSymlinks_RejectsEscapingTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink behavior is flaky on Windows")
	}

	zp := makeZip(t, []zentry{
		{name: "bad-link", mode: os.ModeSymlink | 0o777, data: []byte("../outside")},
	})

	dst := t.TempDir()
	p := zipx.DefaultPolicy()
	p.AllowSymlinks = true

	err := zipx.Unzip(zp, dst, p)
	if err == nil {
		t.Fatalf("expected escaping symlink target error, got nil")
	}
	if !strings.Contains(err.Error(), "symlink target escapes extraction root") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUnzip_AllowSymlinks_RejectsEmptyTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink behavior is flaky on Windows")
	}

	zp := makeZip(t, []zentry{
		{name: "empty-link", mode: os.ModeSymlink | 0o777, data: []byte(" \n\t ")},
	})

	dst := t.TempDir()
	p := zipx.DefaultPolicy()
	p.AllowSymlinks = true

	err := zipx.Unzip(zp, dst, p)
	if err == nil {
		t.Fatalf("expected empty symlink target error, got nil")
	}
	if !strings.Contains(err.Error(), "empty symlink target") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUnzip_SpecialFileSkipped(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("special mode bits are flaky on Windows")
	}

	zp := makeZip(t, []zentry{
		{name: "pipe-entry", mode: os.ModeNamedPipe | 0o644, data: []byte("ignored")},
		{name: "ok.txt", data: []byte("ok")},
	})

	dst := t.TempDir()
	if err := zipx.Unzip(zp, dst, zipx.DefaultPolicy()); err != nil {
		t.Fatalf("Unzip() error: %v", err)
	}

	if _, err := os.Lstat(filepath.Join(dst, "pipe-entry")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected special entry to be skipped, err=%v", err)
	}

	b, err := os.ReadFile(filepath.Join(dst, "ok.txt"))
	if err != nil {
		t.Fatalf("read ok.txt: %v", err)
	}
	if string(b) != "ok" {
		t.Fatalf("ok.txt content mismatch: %q", b)
	}
}

func TestUnzip_PreserveTimes_Directory(t *testing.T) {
	mod := time.Date(2021, 7, 8, 9, 10, 11, 0, time.UTC)

	zp := makeZip(t, []zentry{
		{name: "dir", isDir: true, modified: mod},
	})

	dst := t.TempDir()
	p := zipx.DefaultPolicy()
	p.PreserveTimes = true

	if err := zipx.Unzip(zp, dst, p); err != nil {
		t.Fatalf("Unzip() error: %v", err)
	}

	fi, err := os.Stat(filepath.Join(dst, "dir"))
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if d := fi.ModTime().Sub(mod).Abs(); d > time.Second {
		t.Fatalf("dir mtime not preserved (diff %v): got %v want %v", d, fi.ModTime(), mod)
	}
}
