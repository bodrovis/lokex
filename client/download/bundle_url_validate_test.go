package download_test

import (
	"net"
	"strings"
	"testing"

	"github.com/bodrovis/lokex/v2/client/download"
)

func TestValidateBundleURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr string
	}{
		{
			name:    "empty after trim",
			raw:     "   \t\n   ",
			wantErr: "download: empty url",
		},
		{
			name:    "empty hostname with port only",
			raw:     "https://:443/file.zip",
			wantErr: "download: url has empty hostname",
		},
		{
			name:    "bad url parse",
			raw:     "https://exa mple.com/file.zip",
			wantErr: "download: bad url:",
		},
		{
			name:    "unsupported scheme empty",
			raw:     "//example.com/file.zip",
			wantErr: `download: unsupported url scheme ""`,
		},
		{
			name:    "unsupported scheme http",
			raw:     "http://example.com/file.zip",
			wantErr: `download: unsupported url scheme "http"`,
		},
		{
			name:    "unsupported scheme ftp",
			raw:     "ftp://example.com/file.zip",
			wantErr: `download: unsupported url scheme "ftp"`,
		},
		{
			name: "https scheme is case insensitive",
			raw:  "HTTPS://example.com/file.zip",
			want: "https://example.com/file.zip",
		},
		{
			name:    "empty host",
			raw:     "https:///file.zip",
			wantErr: "download: url has empty host",
		},
		{
			name:    "userinfo with username only not allowed",
			raw:     "https://user@example.com/file.zip",
			wantErr: "download: url must not contain userinfo",
		},
		{
			name:    "userinfo with username and password not allowed",
			raw:     "https://user:pass@example.com/file.zip",
			wantErr: "download: url must not contain userinfo",
		},
		{
			name:    "fragment not allowed",
			raw:     "https://example.com/file.zip#frag",
			wantErr: "download: url must not contain fragment",
		},
		{
			name:    "localhost blocked",
			raw:     "https://localhost/file.zip",
			wantErr: "download: localhost is not allowed",
		},
		{
			name:    "localhost blocked case insensitive",
			raw:     "https://LOCALHOST/file.zip",
			wantErr: "download: localhost is not allowed",
		},
		{
			name:    "subdomain localhost blocked",
			raw:     "https://svc.localhost/file.zip",
			wantErr: "download: local/internal hostname is not allowed",
		},
		{
			name:    "local suffix blocked",
			raw:     "https://printer.local/file.zip",
			wantErr: "download: local/internal hostname is not allowed",
		},
		{
			name:    "internal suffix blocked",
			raw:     "https://api.internal/file.zip",
			wantErr: "download: local/internal hostname is not allowed",
		},
		{
			name:    "private ipv4 10 slash 8 blocked",
			raw:     "https://10.1.2.3/file.zip",
			wantErr: "download: ip 10.1.2.3 is not allowed",
		},
		{
			name:    "private ipv4 172.16 slash 12 blocked",
			raw:     "https://172.16.5.4/file.zip",
			wantErr: "download: ip 172.16.5.4 is not allowed",
		},
		{
			name:    "private ipv4 192.168 slash 16 blocked",
			raw:     "https://192.168.1.10/file.zip",
			wantErr: "download: ip 192.168.1.10 is not allowed",
		},
		{
			name:    "loopback ipv4 blocked",
			raw:     "https://127.0.0.1/file.zip",
			wantErr: "download: ip 127.0.0.1 is not allowed",
		},
		{
			name:    "link local ipv4 blocked",
			raw:     "https://169.254.1.20/file.zip",
			wantErr: "download: ip 169.254.1.20 is not allowed",
		},
		{
			name:    "loopback ipv6 blocked",
			raw:     "https://[::1]/file.zip",
			wantErr: "download: ip ::1 is not allowed",
		},
		{
			name:    "link local ipv6 blocked",
			raw:     "https://[fe80::1]/file.zip",
			wantErr: "download: ip fe80::1 is not allowed",
		},
		{
			name:    "unique local ipv6 blocked",
			raw:     "https://[fc00::1]/file.zip",
			wantErr: "download: ip fc00::1 is not allowed",
		},
		{
			name:    "multicast ipv6 blocked",
			raw:     "https://[ff00::1]/file.zip",
			wantErr: "download: ip ff00::1 is not allowed",
		},
		{
			name:    "unspecified ipv6 blocked",
			raw:     "https://[::]/file.zip",
			wantErr: "download: ip :: is not allowed",
		},
		{
			name: "public ipv4 allowed",
			raw:  "https://8.8.8.8/file.zip",
			want: "https://8.8.8.8/file.zip",
		},
		{
			name: "public ipv6 allowed",
			raw:  "https://[2001:4860:4860::8888]/file.zip",
			want: "https://[2001:4860:4860::8888]/file.zip",
		},
		{
			name: "normal hostname allowed",
			raw:  "https://example.com/file.zip",
			want: "https://example.com/file.zip",
		},
		{
			name: "hostname lowercasing is not forced in result",
			raw:  "https://Example.com/file.zip",
			want: "https://Example.com/file.zip",
		},
		{
			name: "trim input",
			raw:  "  https://example.com/file.zip  ",
			want: "https://example.com/file.zip",
		},
		{
			name: "query preserved",
			raw:  "https://example.com/file.zip?token=abc",
			want: "https://example.com/file.zip?token=abc",
		},
		{
			name: "port preserved",
			raw:  "https://example.com:8443/file.zip",
			want: "https://example.com:8443/file.zip",
		},
		{
			name: "root path allowed",
			raw:  "https://example.com/",
			want: "https://example.com/",
		},
		{
			name: "empty path allowed",
			raw:  "https://example.com",
			want: "https://example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := download.ExportValidateBundleURL(tt.raw)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("ValidateBundleURL() error = nil, want %q", tt.wantErr)
				}
				if strings.HasSuffix(tt.wantErr, ":") {
					if !strings.HasPrefix(err.Error(), tt.wantErr) {
						t.Fatalf("error = %q, want prefix %q", err.Error(), tt.wantErr)
					}
				} else if err.Error() != tt.wantErr {
					t.Fatalf("error = %q, want %q", err.Error(), tt.wantErr)
				}
				if got != "" {
					t.Fatalf("got = %q, want empty string on error", got)
				}
				return
			}

			if err != nil {
				t.Fatalf("ValidateBundleURL() unexpected error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("got = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsBlockedIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ip   net.IP
		want bool
	}{
		{
			name: "nil ip is blocked",
			ip:   nil,
			want: true,
		},
		{
			name: "invalid ip representation is blocked",
			ip:   net.IP{1, 2, 3},
			want: true,
		},
		{
			name: "public ipv4 is allowed",
			ip:   net.ParseIP("8.8.8.8"),
			want: false,
		},
		{
			name: "public ipv6 is allowed",
			ip:   net.ParseIP("2001:4860:4860::8888"),
			want: false,
		},
		{
			name: "private ipv4 is blocked",
			ip:   net.ParseIP("10.1.2.3"),
			want: true,
		},
		{
			name: "loopback ipv4 is blocked",
			ip:   net.ParseIP("127.0.0.1"),
			want: true,
		},
		{
			name: "link local ipv4 is blocked",
			ip:   net.ParseIP("169.254.10.20"),
			want: true,
		},
		{
			name: "loopback ipv6 is blocked",
			ip:   net.ParseIP("::1"),
			want: true,
		},
		{
			name: "link local ipv6 is blocked",
			ip:   net.ParseIP("fe80::1"),
			want: true,
		},
		{
			name: "unique local ipv6 is blocked",
			ip:   net.ParseIP("fc00::1"),
			want: true,
		},
		{
			name: "multicast ipv6 is blocked",
			ip:   net.ParseIP("ff00::1"),
			want: true,
		},
		{
			name: "unspecified ipv6 is blocked",
			ip:   net.ParseIP("::"),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := download.ExportIsBlockedIP(tt.ip)
			if got != tt.want {
				t.Fatalf("IsBlockedIP(%v) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

func TestMustCIDR(t *testing.T) {
	t.Parallel()

	t.Run("valid cidr", func(t *testing.T) {
		t.Parallel()

		got := download.ExportMustCIDR("10.0.0.0/8")
		if got == nil {
			t.Fatal("MustCIDR() = nil, want non-nil")
		}
		if got.String() != "10.0.0.0/8" {
			t.Fatalf("MustCIDR() = %q, want %q", got.String(), "10.0.0.0/8")
		}
	})

	t.Run("invalid cidr panics", func(t *testing.T) {
		t.Parallel()

		defer func() {
			if r := recover(); r == nil {
				t.Fatal("MustCIDR() panic = nil, want non-nil panic")
			}
		}()

		_ = download.ExportMustCIDR("definitely-not-a-cidr")
	})
}
