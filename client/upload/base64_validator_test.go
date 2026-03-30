package upload_test

import (
	"strings"
	"testing"

	"github.com/bodrovis/lokex/v2/client/upload"
)

func TestValidateAndNormalizeStdBase64String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		in      string
		want    string
		wantErr string
	}{
		{
			name:    "empty after trim",
			in:      "   \t\n   ",
			wantErr: "upload: 'data' cannot be empty",
		},
		{
			name:    "len mod 4 equals 1 invalid",
			in:      "A",
			wantErr: "upload: 'data' base64 length is invalid (len%4==1)",
		},
		{
			name: "valid base64 no padding needed",
			in:   "QUJDRA==", // ABCD
			want: "QUJDRA==",
		},
		{
			name: "valid base64 no padding provided mod 2",
			in:   "QUI", // AB -> needs =
			want: "QUI=",
		},
		{
			name: "valid base64 no padding provided mod 3",
			in:   "QUJD", // already mod 0
			want: "QUJD",
		},
		{
			name:    "invalid char",
			in:      "QUJD$A==",
			wantErr: "upload: 'data' contains non-base64 char '$'",
		},
		{
			name:    "padding in middle",
			in:      "QU=J",
			wantErr: "upload: invalid base64 padding position",
		},
		{
			name:    "too many padding chars",
			in:      "QUJD===",
			wantErr: "upload: invalid base64 padding",
		},
		{
			name:    "padding but length not multiple of 4",
			in:      "QU=",
			wantErr: "upload: invalid base64 padding (length must be multiple of 4 when '=' present)",
		},
		{
			name:    "padding not at end",
			in:      "QUJD=A==",
			wantErr: "upload: invalid base64 padding",
		},
		{
			name: "valid single padding",
			in:   "QUI=",
			want: "QUI=",
		},
		{
			name: "valid double padding",
			in:   "QQ==",
			want: "QQ==",
		},
		{
			name: "trim spaces",
			in:   "   QUI=   ",
			want: "QUI=",
		},
		{
			name: "alphabet edge chars",
			in:   "AZaz09+/",
			want: "AZaz09+/",
		},
		{
			name: "no padding normalize mod 3",
			in:   "QUJDRA", // len%4=2 → add ==
			want: "QUJDRA==",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := upload.ExportValidateAndNormalizeStdBase64String(tt.in)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("error = nil, want %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %q, want contain %q", err.Error(), tt.wantErr)
				}
				if got != "" {
					t.Fatalf("got = %q, want empty", got)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("got = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeStdBase64Padding_InvalidPaddingAtEnd(t *testing.T) {
	t.Parallel()

	_, err := upload.ExportNormalizeStdBase64Padding("ABCX", 1)
	if err == nil {
		t.Fatal("error = nil, want error")
	}
	if err.Error() != "upload: invalid base64 padding" {
		t.Fatalf("error = %q, want %q", err.Error(), "upload: invalid base64 padding")
	}
}
