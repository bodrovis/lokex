package upload_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bodrovis/lokex/v2/client/upload"
)

func TestEnsureFileIsRegular_NotRegularFile(t *testing.T) {
	t.Parallel()

	err := upload.ExportEnsureFileIsRegular(os.DevNull)
	if err == nil {
		t.Fatal("EnsureFileIsRegular() error = nil, want non-nil")
	}

	want := fmt.Sprintf("upload: %q is not a regular file", os.DevNull)
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
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
			path:    "   \t\n   ",
			wantErr: "upload: empty file path",
		},
		{
			name:    "file not found",
			path:    filepath.Join(tmpDir, "missing.txt"),
			wantErr: "upload: file not found:",
		},
		{
			name:    "directory",
			path:    subdir,
			wantErr: "is a directory, need a file",
		},
		{
			name: "regular file",
			path: regularFile,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := upload.ExportEnsureFileIsRegular(tt.path)

			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("EnsureFileIsRegular() unexpected error = %v", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("EnsureFileIsRegular() error = nil, want %q", tt.wantErr)
			}

			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}
