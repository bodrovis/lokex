package zipx_test

import (
	"bytes"
	"errors"
	"io"
	"math"
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

func TestCopyWithLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		src        io.Reader
		max        int64
		enabled    bool
		wantN      int64
		wantErr    string
		wantOutput string
	}{
		{
			name:       "within limit",
			src:        strings.NewReader("abc"),
			max:        5,
			enabled:    true,
			wantN:      3,
			wantOutput: "abc",
		},
		{
			name:       "exact limit",
			src:        strings.NewReader("abc"),
			max:        3,
			enabled:    true,
			wantN:      3,
			wantOutput: "abc",
		},
		{
			name:       "limit exceeded",
			src:        strings.NewReader("abcd"),
			max:        3,
			enabled:    true,
			wantN:      3,
			wantErr:    "zip entry exceeds max size",
			wantOutput: "abc",
		},
		{
			name:       "zero limit empty input",
			src:        strings.NewReader(""),
			max:        0,
			enabled:    true,
			wantN:      0,
			wantOutput: "",
		},
		{
			name:       "zero limit exceeded",
			src:        strings.NewReader("abc"),
			max:        0,
			enabled:    true,
			wantN:      0,
			wantErr:    "zip entry exceeds max size",
			wantOutput: "",
		},
		{
			name:       "disabled limit",
			src:        strings.NewReader("abcd"),
			max:        2,
			enabled:    false,
			wantN:      4,
			wantOutput: "abcd",
		},
		{
			name:       "disabled zero limit",
			src:        strings.NewReader("abc"),
			max:        0,
			enabled:    false,
			wantN:      3,
			wantOutput: "abc",
		},
		{
			name:       "read error with limit",
			src:        errReader{err: errors.New("read boom")},
			max:        3,
			enabled:    true,
			wantN:      0,
			wantErr:    "read boom",
			wantOutput: "",
		},
		{
			name:       "read error without limit",
			src:        errReader{err: errors.New("plain boom")},
			enabled:    false,
			wantN:      0,
			wantErr:    "plain boom",
			wantOutput: "",
		},
		{
			name:       "max int64",
			src:        strings.NewReader("abc"),
			max:        math.MaxInt64,
			enabled:    true,
			wantN:      3,
			wantOutput: "abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var dst bytes.Buffer

			gotN, err := zipx.ExportCopyWithLimit(
				&dst,
				tt.src,
				tt.max,
				tt.enabled,
			)

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
