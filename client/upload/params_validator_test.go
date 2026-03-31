package upload_test

import (
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
		check    func(t *testing.T, got upload.UploadParams)
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
			name:     "success trims filename and clones map",
			params:   upload.UploadParams{"filename": "  file.json  ", "lang_iso": "en"},
			wantName: "file.json",
			check: func(t *testing.T, got upload.UploadParams) {
				t.Helper()

				if got["filename"] != "file.json" {
					t.Fatalf("filename = %v, want %q", got["filename"], "file.json")
				}
				if got["lang_iso"] != "en" {
					t.Fatalf("lang_iso = %v, want %q", got["lang_iso"], "en")
				}
			},
		},
		{
			name:     "success with only filename",
			params:   upload.UploadParams{"filename": "a.txt"},
			wantName: "a.txt",
			check: func(t *testing.T, got upload.UploadParams) {
				t.Helper()

				if got["filename"] != "a.txt" {
					t.Fatalf("filename = %v, want %q", got["filename"], "a.txt")
				}
				if len(got) != 1 {
					t.Fatalf("len(got) = %d, want %d", len(got), 1)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			orig := make(upload.UploadParams, len(tt.params))
			for k, v := range tt.params {
				orig[k] = v
			}

			got, gotName, err := upload.ExportCloneAndValidateParams(tt.params)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("CloneAndValidateParams() error = nil, want %q", tt.wantErr)
				}
				if err.Error() != tt.wantErr {
					t.Fatalf("error = %q, want %q", err.Error(), tt.wantErr)
				}
				if got != nil {
					t.Fatalf("got = %#v, want nil on error", got)
				}
				if gotName != "" {
					t.Fatalf("gotName = %q, want empty string on error", gotName)
				}
				return
			}

			if err != nil {
				t.Fatalf("CloneAndValidateParams() unexpected error = %v", err)
			}
			if gotName != tt.wantName {
				t.Fatalf("gotName = %q, want %q", gotName, tt.wantName)
			}
			if got == nil {
				t.Fatal("got map = nil, want non-nil")
			}
			if tt.check != nil {
				tt.check(t, got)
			}

			// Ensure original params map was not mutated.
			for k, v := range orig {
				if tt.params[k] != v {
					t.Fatalf("original params mutated: params[%q] = %v, want %v", k, tt.params[k], v)
				}
			}

			// Ensure returned map is a clone, not the same map.
			got["filename"] = "changed.txt"
			if tt.params["filename"] == "changed.txt" {
				t.Fatal("original params map was affected by modifying returned map")
			}
		})
	}
}
