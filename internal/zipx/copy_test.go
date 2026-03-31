package zipx_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/bodrovis/lokex/v2/internal/zipx"
)

type errReader struct {
	err error
}

func (r errReader) Read(_ []byte) (int, error) {
	return 0, r.err
}

func TestCopyCapped(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		src        io.Reader
		max        int64
		wantN      int64
		wantErr    string
		wantOutput string
	}{
		{
			name:       "max positive within limit",
			src:        strings.NewReader("abc"),
			max:        3,
			wantN:      3,
			wantOutput: "abc",
		},
		{
			name:       "max positive exceeded",
			src:        strings.NewReader("abcd"),
			max:        3,
			wantN:      4,
			wantErr:    "zip entry exceeds max size",
			wantOutput: "abcd",
		},
		{
			name:       "copy error with max positive",
			src:        errReader{err: errors.New("read boom")},
			max:        3,
			wantN:      0,
			wantErr:    "read boom",
			wantOutput: "",
		},
		{
			name:       "max zero uses plain copy",
			src:        strings.NewReader("abc"),
			max:        0,
			wantN:      3,
			wantOutput: "abc",
		},
		{
			name:       "max negative uses plain copy",
			src:        strings.NewReader("abcd"),
			max:        -1,
			wantN:      4,
			wantOutput: "abcd",
		},
		{
			name:       "plain copy error",
			src:        errReader{err: errors.New("plain boom")},
			max:        -1,
			wantN:      0,
			wantErr:    "plain boom",
			wantOutput: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var dst bytes.Buffer

			gotN, err := zipx.ExportCopyCapped(&dst, tt.src, tt.max)

			if gotN != tt.wantN {
				t.Fatalf("n = %d, want %d", gotN, tt.wantN)
			}

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("error = nil, want %q", tt.wantErr)
				}
				if err.Error() != tt.wantErr {
					t.Fatalf("error = %q, want %q", err.Error(), tt.wantErr)
				}
			} else if err != nil {
				t.Fatalf("unexpected error = %v", err)
			}

			if dst.String() != tt.wantOutput {
				t.Fatalf("output = %q, want %q", dst.String(), tt.wantOutput)
			}
		})
	}
}
