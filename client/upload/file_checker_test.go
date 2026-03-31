package upload_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bodrovis/lokex/v2/client/upload"
)

type fakeFileInfo struct {
	name string
	size int64
	mode os.FileMode
}

func (f fakeFileInfo) Name() string       { return f.name }
func (f fakeFileInfo) Size() int64        { return f.size }
func (f fakeFileInfo) Mode() os.FileMode  { return f.mode }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return f.mode.IsDir() }
func (f fakeFileInfo) Sys() any           { return nil }

func TestEnsureFileIsRegular_StatError(t *testing.T) {
	restore := upload.ExportSetStatFileForTest(func(string) (os.FileInfo, error) {
		return nil, errors.New("stat boom")
	})
	defer restore()

	err := upload.ExportEnsureFileIsRegular("/tmp/file.txt")
	if err == nil {
		t.Fatal("EnsureFileIsRegular() error = nil, want non-nil")
	}
	if err.Error() != `upload: stat "/tmp/file.txt": stat boom` {
		t.Fatalf("error = %q, want %q", err.Error(), `upload: stat "/tmp/file.txt": stat boom`)
	}
}

func TestEnsureFileIsRegular_NotRegularFile(t *testing.T) {
	restore := upload.ExportSetStatFileForTest(func(path string) (os.FileInfo, error) {
		return fakeFileInfo{
			name: path,
			mode: os.ModeNamedPipe,
		}, nil
	})
	defer restore()

	err := upload.ExportEnsureFileIsRegular("/tmp/weird")
	if err == nil {
		t.Fatal("EnsureFileIsRegular() error = nil, want non-nil")
	}
	if err.Error() != `upload: "/tmp/weird" is not a regular file` {
		t.Fatalf("error = %q, want %q", err.Error(), `upload: "/tmp/weird" is not a regular file`)
	}
}

func TestEnsureFileIsRegular(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	regularFile := filepath.Join(tmpDir, "file.txt")
	if err := os.WriteFile(regularFile, []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	subdir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}

	tests := []struct {
		name    string
		path    string
		wantErr string
	}{
		{
			name:    "empty path",
			path:    "   \t\n  ",
			wantErr: "upload: empty file path",
		},
		{
			name:    "file not found",
			path:    filepath.Join(tmpDir, "missing.txt"),
			wantErr: `upload: file not found: "`,
		},
		{
			name:    "directory",
			path:    subdir,
			wantErr: `is a directory, need a file`,
		},
		{
			name:    "regular file",
			path:    regularFile,
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := upload.ExportEnsureFileIsRegular(tt.path)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("EnsureFileIsRegular() error = nil, want %q", tt.wantErr)
				}

				if strings.HasPrefix(tt.wantErr, `upload: file not found: "`) {
					if !strings.HasPrefix(err.Error(), tt.wantErr) {
						t.Fatalf("error = %q, want prefix %q", err.Error(), tt.wantErr)
					}
				} else if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("EnsureFileIsRegular() unexpected error = %v", err)
			}
		})
	}
}
