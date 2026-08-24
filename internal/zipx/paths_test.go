package zipx_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/bodrovis/lokex/v2/internal/zipx"
)

func TestNormalizeZipEntryPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{
			name:  "plain file",
			input: "a.txt",
			want:  "a.txt",
		},
		{
			name:  "nested",
			input: "dir/file.txt",
			want:  filepath.Join("dir", "file.txt"),
		},
		{
			name:  "backslashes normalized",
			input: `dir\file.txt`,
			want:  filepath.Join("dir", "file.txt"),
		},
		{
			name:  "strip leading dot slash",
			input: "./dir/file.txt",
			want:  filepath.Join("dir", "file.txt"),
		},
		{
			name:  "collapse duplicate slashes",
			input: "dir//file.txt",
			want:  filepath.Join("dir", "file.txt"),
		},
		{
			name:  "clean inner dot",
			input: "a/./b.txt",
			want:  filepath.Join("a", "b.txt"),
		},
		{
			name:  "clean inner parent remains safe",
			input: "a/../b.txt",
			want:  "b.txt",
		},
		{
			name:  "empty becomes empty",
			input: "",
			want:  "",
		},
		{
			name:  "dot becomes empty",
			input: ".",
			want:  "",
		},
		{
			name:  "dot slash becomes empty",
			input: "./",
			want:  "",
		},
		{
			name:    "absolute path rejected",
			input:   "/dir/file.txt",
			wantErr: `unsafe absolute path in zip: "/dir/file.txt"`,
		},
		{
			name:    "root path rejected",
			input:   "/",
			wantErr: `unsafe absolute path in zip: "/"`,
		},
		{
			name:    "parent traversal",
			input:   "../x.txt",
			wantErr: `unsafe path traversal in zip (.. segment): "../x.txt"`,
		},
		{
			name:    "deep parent traversal",
			input:   "a/../../x.txt",
			wantErr: `unsafe path traversal in zip (.. segment): "a/../../x.txt"`,
		},
		{
			name:    "just parent",
			input:   "..",
			wantErr: `unsafe path traversal in zip (.. segment): ".."`,
		},
		{
			name:    "parent with slash",
			input:   "../",
			wantErr: `unsafe path traversal in zip (.. segment): "../"`,
		},
		{
			name:    "nul in name",
			input:   "a\x00b",
			wantErr: `invalid file name (NUL) in zip: "a\x00b"`,
		},
		{
			name:    "only nul",
			input:   "\x00",
			wantErr: `invalid file name (NUL) in zip: "\x00"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := zipx.ExportNormalizeZipEntryPath(tt.input)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf(
						"NormalizeZipEntryPath(%q) error = nil, want %q",
						tt.input,
						tt.wantErr,
					)
				}

				if err.Error() != tt.wantErr {
					t.Fatalf(
						"error = %q, want %q",
						err.Error(),
						tt.wantErr,
					)
				}

				return
			}

			if err != nil {
				t.Fatalf(
					"NormalizeZipEntryPath(%q) unexpected error = %v",
					tt.input,
					err,
				)
			}

			if got != tt.want {
				t.Fatalf(
					"NormalizeZipEntryPath(%q) = %q, want %q",
					tt.input,
					got,
					tt.want,
				)
			}
		})
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
		if err != nil {
			return
		}

		if got == "." {
			t.Fatalf("got %q, want normalized empty instead of dot", got)
		}

		if got != "" && !filepath.IsLocal(got) {
			t.Fatalf("got non-local path %q", got)
		}

		if filepath.IsAbs(got) {
			t.Fatalf("got absolute path %q", got)
		}

		for _, seg := range strings.Split(filepath.ToSlash(got), "/") {
			if seg == ".." {
				t.Fatalf("got %q contains parent traversal segment", got)
			}
		}
	})
}
