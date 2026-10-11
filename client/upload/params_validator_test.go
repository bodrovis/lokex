package upload_test

import (
	"maps"
	"testing"

	"github.com/bodrovis/lokex/v2/client/upload"
)

func TestCloneAndValidateParams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		params   upload.UploadParams
		wantName string
		wantErr  string
	}{
		{
			name:    "missing filename",
			params:  upload.UploadParams{"lang_iso": "en"},
			wantErr: "upload: missing 'filename' param",
		},
		{
			name:    "filename wrong type",
			params:  upload.UploadParams{"filename": 123},
			wantErr: "upload: 'filename' must be a non-empty string",
		},
		{
			name:    "filename empty string",
			params:  upload.UploadParams{"filename": ""},
			wantErr: "upload: 'filename' must be a non-empty string",
		},
		{
			name:    "filename whitespace only",
			params:  upload.UploadParams{"filename": "   \t\n  "},
			wantErr: "upload: 'filename' must be a non-empty string",
		},
		{
			name: "success with only filename",
			params: upload.UploadParams{
				"filename": "a.txt",
			},
			wantName: "a.txt",
		},
		{
			name: "success preserves filename whitespace",
			params: upload.UploadParams{
				"filename": "  file.json  ",
				"lang_iso": "en",
			},
			wantName: "  file.json  ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			orig := maps.Clone(tt.params)

			got, gotName, err := upload.ExportCloneAndValidateParams(tt.params)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("error = nil, want %q", tt.wantErr)
				}
				if err.Error() != tt.wantErr {
					t.Fatalf("error = %q, want %q", err, tt.wantErr)
				}
				if got != nil {
					t.Fatalf("got = %#v, want nil on error", got)
				}
				if gotName != "" {
					t.Fatalf("gotName = %q, want empty", gotName)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error = %v", err)
				}
				if gotName != tt.wantName {
					t.Fatalf("gotName = %q, want %q", gotName, tt.wantName)
				}
				if got == nil {
					t.Fatal("got map = nil, want non-nil")
				}
				if !maps.Equal(got, tt.params) {
					t.Fatalf("got = %#v, want %#v", got, tt.params)
				}

				// The returned map must be independent.
				got["filename"] = "changed.txt"
			}

			// The original map must remain unchanged.
			if !maps.Equal(tt.params, orig) {
				t.Fatalf(
					"original params mutated: got %#v, want %#v",
					tt.params,
					orig,
				)
			}
		})
	}
}
