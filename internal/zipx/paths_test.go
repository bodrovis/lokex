package zipx_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bodrovis/lokex/v2/internal/zipx"
)

type fakeFileInfo struct {
	mode os.FileMode
}

func (fi fakeFileInfo) Name() string       { return "" }
func (fi fakeFileInfo) Size() int64        { return 0 }
func (fi fakeFileInfo) Mode() os.FileMode  { return fi.mode }
func (fi fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (fi fakeFileInfo) IsDir() bool        { return fi.mode.IsDir() }
func (fi fakeFileInfo) Sys() any           { return nil }

func TestIsPathWithinBase(t *testing.T) {
	base := filepath.Clean(filepath.Join(string(filepath.Separator), "tmp", "base"))

	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "same path",
			path: base,
			want: true,
		},
		{
			name: "child path",
			path: filepath.Join(base, "a", "b.txt"),
			want: true,
		},
		{
			name: "sibling outside",
			path: filepath.Clean(filepath.Join(base, "..", "other", "x.txt")),
			want: false,
		},
		{
			name: "parent outside",
			path: filepath.Dir(base),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := zipx.ExportIsPathWithinBase(base, tt.path)
			if got != tt.want {
				t.Fatalf("IsPathWithinBase(%q, %q) = %v, want %v", base, tt.path, got, tt.want)
			}
		})
	}
}

func TestNormalizeZipEntryPath(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{"plain file", "a.txt", "a.txt", ""},
		{"nested", "dir/file.txt", "dir/file.txt", ""},
		{"backslashes normalized", `dir\file.txt`, "dir/file.txt", ""},
		{"strip leading dot slash", "./dir/file.txt", "dir/file.txt", ""},
		{"strip leading slash", "/dir/file.txt", "dir/file.txt", ""},
		{"collapse duplicate slashes", "////dir//file.txt", "dir/file.txt", ""},
		{"clean inner dot", "a/./b.txt", "a/b.txt", ""},
		{"clean inner parent remains safe", "a/../b.txt", "b.txt", ""},
		{"empty becomes empty", "", "", ""},
		{"dot becomes empty", ".", "", ""},
		{"dot slash becomes empty", "./", "", ""},
		{"slash becomes empty", "/", "", ""},
		{"parent traversal", "../x.txt", "", `unsafe path traversal in zip (.. segment): "../x.txt"`},
		{"deep parent traversal", "a/../../x.txt", "", `unsafe path traversal in zip (.. segment): "a/../../x.txt"`},
		{"just parent", "..", "", `unsafe path traversal in zip (.. segment): ".."`},
		{"parent with slash", "../", "", `unsafe path traversal in zip (.. segment): "../"`},
		{"nul in name", "a\x00b", "", `invalid file name (NUL) in zip: "a\x00b"`},
		{"only nul", "\x00", "", `invalid file name (NUL) in zip: "\x00"`},
		{"windows drive style preserved here", `C:\temp\a.txt`, "C:/temp/a.txt", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := zipx.ExportNormalizeZipEntryPath(tt.input)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("NormalizeZipEntryPath(%q) error = nil, want %q", tt.input, tt.wantErr)
				}
				if err.Error() != tt.wantErr {
					t.Fatalf("error = %q, want %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeZipEntryPath(%q) unexpected error = %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("NormalizeZipEntryPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestResolveTargetPath(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		destDir := t.TempDir()
		destReal, err := filepath.Abs(destDir)
		if err != nil {
			t.Fatal(err)
		}

		got, err := zipx.ExportResolveTargetPath(destDir, destReal, "dir/file.txt", "dir/file.txt")
		if err != nil {
			t.Fatalf("ResolveTargetPath() unexpected error = %v", err)
		}

		want := filepath.Join(destDir, "dir", "file.txt")
		want, err = filepath.Abs(want)
		if err != nil {
			t.Fatal(err)
		}

		if got != want {
			t.Fatalf("target = %q, want %q", got, want)
		}
	})

	t.Run("absolute path rejected", func(t *testing.T) {
		destDir := t.TempDir()
		destReal, err := filepath.Abs(destDir)
		if err != nil {
			t.Fatal(err)
		}

		var input string
		if filepath.Separator == '\\' {
			input = `C:\etc\passwd`
		} else {
			input = `/etc/passwd`
		}

		_, err = zipx.ExportResolveTargetPath(destDir, destReal, input, input)
		if err == nil {
			t.Fatal("ResolveTargetPath() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "unsafe absolute path in zip") {
			t.Fatalf("error = %q, want absolute path error", err.Error())
		}
	})

	t.Run("abs error is returned", func(t *testing.T) {
		restore := zipx.ExportSetAbsFilepathForTest(func(string) (string, error) {
			return "", errors.New("abs boom")
		})
		defer restore()

		_, err := zipx.ExportResolveTargetPath("dest", "dest", "a.txt", "a.txt")
		if err == nil {
			t.Fatal("ResolveTargetPath() error = nil, want non-nil")
		}
		if err.Error() != "abs boom" {
			t.Fatalf("error = %q, want %q", err.Error(), "abs boom")
		}
	})

	t.Run("path escape rejected", func(t *testing.T) {
		restore := zipx.ExportSetIsPathWithinBaseForTest(func(string, string) bool {
			return false
		})
		defer restore()

		destDir := t.TempDir()
		destReal, err := filepath.Abs(destDir)
		if err != nil {
			t.Fatal(err)
		}

		_, err = zipx.ExportResolveTargetPath(destDir, destReal, "a.txt", "orig.txt")
		if err == nil {
			t.Fatal("ResolveTargetPath() error = nil, want non-nil")
		}
		if err.Error() != `unsafe path escape: "orig.txt"` {
			t.Fatalf("error = %q, want %q", err.Error(), `unsafe path escape: "orig.txt"`)
		}
	})
}

func TestPathHasSymlinkOutside_NoSymlinks(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "a", "b", "file.txt")

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := zipx.ExportPathHasSymlinkOutside(root, target)
	if err != nil {
		t.Fatalf("PathHasSymlinkOutside() unexpected error = %v", err)
	}
	if got {
		t.Fatal("PathHasSymlinkOutside() = true, want false")
	}
}

func TestPathHasSymlinkOutside_MissingIntermediateIgnored(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "not", "existing", "file.txt")

	got, err := zipx.ExportPathHasSymlinkOutside(root, target)
	if err != nil {
		t.Fatalf("PathHasSymlinkOutside() unexpected error = %v", err)
	}
	if got {
		t.Fatal("PathHasSymlinkOutside() = true, want false")
	}
}

func TestPathHasSymlinkOutside_SymlinkInsideRoot(t *testing.T) {
	root := t.TempDir()

	realDir := filepath.Join(root, "real")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatal(err)
	}

	link := filepath.Join(root, "link")
	if err := os.Symlink(realDir, link); err != nil {
		t.Skipf("symlink not supported on this platform: %v", err)
	}

	target := filepath.Join(link, "file.txt")

	got, err := zipx.ExportPathHasSymlinkOutside(root, target)
	if err != nil {
		t.Fatalf("PathHasSymlinkOutside() unexpected error = %v", err)
	}
	if got {
		t.Fatal("PathHasSymlinkOutside() = true, want false")
	}
}

func TestPathHasSymlinkOutside_SymlinkOutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()

	link := filepath.Join(root, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink not supported on this platform: %v", err)
	}

	target := filepath.Join(link, "file.txt")

	got, err := zipx.ExportPathHasSymlinkOutside(root, target)
	if err != nil {
		t.Fatalf("PathHasSymlinkOutside() unexpected error = %v", err)
	}
	if !got {
		t.Fatal("PathHasSymlinkOutside() = false, want true")
	}
}

func TestPathHasSymlinkOutside_SymlinkExactlyToRootIsAllowed(t *testing.T) {
	root := t.TempDir()

	link := filepath.Join(root, "link")
	if err := os.Symlink(root, link); err != nil {
		t.Skipf("symlink not supported on this platform: %v", err)
	}

	target := filepath.Join(link, "file.txt")

	got, err := zipx.ExportPathHasSymlinkOutside(root, target)
	if err != nil {
		t.Fatalf("PathHasSymlinkOutside() unexpected error = %v", err)
	}
	if got {
		t.Fatal("PathHasSymlinkOutside() = true, want false")
	}
}

func TestPathHasSymlinkOutside_RelError(t *testing.T) {
	restore := zipx.ExportSetFilepathRelForTest(func(string, string) (string, error) {
		return "", errors.New("rel boom")
	})
	defer restore()

	got, err := zipx.ExportPathHasSymlinkOutside("root", "file")
	if err == nil {
		t.Fatal("PathHasSymlinkOutside() error = nil, want non-nil")
	}
	if !got {
		t.Fatal("got = false, want true on rel error")
	}
	if err.Error() != "rel boom" {
		t.Fatalf("error = %q, want %q", err.Error(), "rel boom")
	}
}

func TestPathHasSymlinkOutside_LstatError(t *testing.T) {
	restoreRel := zipx.ExportSetFilepathRelForTest(func(string, string) (string, error) {
		return filepath.Join("a", "b"), nil
	})
	defer restoreRel()

	restoreLstat := zipx.ExportSetLstatForTest(func(string) (os.FileInfo, error) {
		return nil, errors.New("lstat boom")
	})
	defer restoreLstat()

	got, err := zipx.ExportPathHasSymlinkOutside("root", filepath.Join("root", "a", "b"))
	if err == nil {
		t.Fatal("PathHasSymlinkOutside() error = nil, want non-nil")
	}
	if got {
		t.Fatal("got = true, want false for lstat non-rel error")
	}
	if err.Error() != "lstat boom" {
		t.Fatalf("error = %q, want %q", err.Error(), "lstat boom")
	}
}

func TestPathHasSymlinkOutside_EvalSymlinksError(t *testing.T) {
	restoreRel := zipx.ExportSetFilepathRelForTest(func(string, string) (string, error) {
		return "a", nil
	})
	defer restoreRel()

	restoreLstat := zipx.ExportSetLstatForTest(func(string) (os.FileInfo, error) {
		return fakeFileInfo{mode: os.ModeSymlink}, nil
	})
	defer restoreLstat()

	restoreEval := zipx.ExportSetEvalSymlinksPathForTest(func(string) (string, error) {
		return "", errors.New("eval boom")
	})
	defer restoreEval()

	got, err := zipx.ExportPathHasSymlinkOutside("root", filepath.Join("root", "a"))
	if err == nil {
		t.Fatal("PathHasSymlinkOutside() error = nil, want non-nil")
	}
	if !got {
		t.Fatal("got = false, want true on eval error")
	}
	if err.Error() != "eval boom" {
		t.Fatalf("error = %q, want %q", err.Error(), "eval boom")
	}
}

func TestPathHasSymlinkOutside_MockedResolvedOutside(t *testing.T) {
	restoreRel := zipx.ExportSetFilepathRelForTest(func(string, string) (string, error) {
		return filepath.Join("a", "b"), nil
	})
	defer restoreRel()

	calls := 0
	restoreLstat := zipx.ExportSetLstatForTest(func(string) (os.FileInfo, error) {
		calls++
		if calls == 1 {
			return fakeFileInfo{mode: os.ModeSymlink}, nil
		}
		return nil, os.ErrNotExist
	})
	defer restoreLstat()

	restoreEval := zipx.ExportSetEvalSymlinksPathForTest(func(string) (string, error) {
		return filepath.Join(string(filepath.Separator), "outside"), nil
	})
	defer restoreEval()

	got, err := zipx.ExportPathHasSymlinkOutside("root", filepath.Join("root", "a", "b"))
	if err != nil {
		t.Fatalf("PathHasSymlinkOutside() unexpected error = %v", err)
	}
	if !got {
		t.Fatal("got = false, want true")
	}
}

func TestPathHasSymlinkOutside_MockedResolvedInside(t *testing.T) {
	restoreRel := zipx.ExportSetFilepathRelForTest(func(string, string) (string, error) {
		return "a", nil
	})
	defer restoreRel()

	restoreLstat := zipx.ExportSetLstatForTest(func(string) (os.FileInfo, error) {
		return fakeFileInfo{mode: os.ModeSymlink}, nil
	})
	defer restoreLstat()

	restoreEval := zipx.ExportSetEvalSymlinksPathForTest(func(string) (string, error) {
		return filepath.Join("root", "sub"), nil
	})
	defer restoreEval()

	got, err := zipx.ExportPathHasSymlinkOutside("root", filepath.Join("root", "a"))
	if err != nil {
		t.Fatalf("PathHasSymlinkOutside() unexpected error = %v", err)
	}
	if got {
		t.Fatal("got = true, want false")
	}
}

func FuzzNormalizeZipEntryPath(f *testing.F) {
	seeds := []string{
		"",
		".",
		"./a",
		"../a",
		`a\b\c`,
		"/a/b",
		"a/../b",
		"a\x00b",
		"////",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		got, err := zipx.ExportNormalizeZipEntryPath(input)
		if err == nil {
			if strings.Contains(got, `\`) {
				t.Fatalf("got %q contains backslash", got)
			}
			if got == "." {
				t.Fatalf("got %q, want normalized empty instead of dot", got)
			}
			for seg := range strings.SplitSeq(got, "/") {
				if seg == ".." {
					t.Fatalf("got %q contains parent traversal segment", got)
				}
			}
		}
	})
}

func TestIsPathWithinBase_RelError(t *testing.T) {
	restore := zipx.ExportSetFilepathRelForTest(func(string, string) (string, error) {
		return "", errors.New("rel boom")
	})
	defer restore()

	got := zipx.ExportIsPathWithinBase("base", "path")
	if got {
		t.Fatal("IsPathWithinBase() = true, want false on rel error")
	}
}

func TestPathHasSymlinkOutside_SkipsEmptySegment(t *testing.T) {
	sep := string(filepath.Separator)

	restoreRel := zipx.ExportSetFilepathRelForTest(func(string, string) (string, error) {
		return "a" + sep + sep + "b", nil
	})
	defer restoreRel()

	var seen []string
	restoreLstat := zipx.ExportSetLstatForTest(func(name string) (os.FileInfo, error) {
		seen = append(seen, name)
		return nil, os.ErrNotExist
	})
	defer restoreLstat()

	got, err := zipx.ExportPathHasSymlinkOutside("root", filepath.Join("root", "a", "b"))
	if err != nil {
		t.Fatalf("PathHasSymlinkOutside() unexpected error = %v", err)
	}
	if got {
		t.Fatal("PathHasSymlinkOutside() = true, want false")
	}

	want := []string{
		filepath.Join("root", "a"),
		filepath.Join("root", "a", "b"),
	}
	if len(seen) != len(want) {
		t.Fatalf("lstat calls = %v, want %v", seen, want)
	}
	for i := range want {
		if seen[i] != want[i] {
			t.Fatalf("lstat calls = %v, want %v", seen, want)
		}
	}
}
