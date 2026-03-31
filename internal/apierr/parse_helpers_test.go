package apierr_test

import (
	"testing"

	"github.com/bodrovis/lokex/v2/internal/apierr"
)

func TestCoalesce(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ss   []string
		want string
	}{
		{
			name: "returns first non empty string",
			ss:   []string{"", "first", "second"},
			want: "first",
		},
		{
			name: "returns empty when all strings are empty",
			ss:   []string{"", "", ""},
			want: "",
		},
		{
			name: "returns empty when no args",
			ss:   nil,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := apierr.ExportCoalesce(tt.ss...)
			if got != tt.want {
				t.Fatalf("Coalesce(%q) = %q, want %q", tt.ss, got, tt.want)
			}
		})
	}
}

func TestGetNumberAsInt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		m    map[string]any
		key  string
		want int
		ok   bool
	}{
		{
			name: "float64 value",
			m: map[string]any{
				"status": float64(429),
			},
			key:  "status",
			want: 429,
			ok:   true,
		},
		{
			name: "int value",
			m: map[string]any{
				"status": 503,
			},
			key:  "status",
			want: 503,
			ok:   true,
		},
		{
			name: "missing key",
			m:    map[string]any{},
			key:  "status",
			want: 0,
			ok:   false,
		},
		{
			name: "unsupported type",
			m: map[string]any{
				"status": true,
			},
			key:  "status",
			want: 0,
			ok:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := apierr.ExportGetNumberAsInt(tt.m, tt.key)
			if ok != tt.ok {
				t.Fatalf("GetNumberAsInt(%v, %q) ok = %v, want %v", tt.m, tt.key, ok, tt.ok)
			}
			if got != tt.want {
				t.Fatalf("GetNumberAsInt(%v, %q) = %d, want %d", tt.m, tt.key, got, tt.want)
			}
		})
	}
}
