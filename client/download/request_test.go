package download_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/bodrovis/lokex/v2/client"
	"github.com/bodrovis/lokex/v2/client/download"
)

func TestDoDownloadRequest(t *testing.T) {
	t.Parallel()

	t.Run("bad url returns build request error", func(t *testing.T) {
		t.Parallel()

		d := download.NewDownloader(&client.Client{
			HTTPClient: &http.Client{},
		})

		resp, err := download.ExportDoDownloadRequest(
			d,
			context.Background(),
			&http.Client{},
			"://bad url",
			"",
		)
		if err == nil {
			t.Fatal("DoDownloadRequest() error = nil, want non-nil")
		}
		if !strings.HasPrefix(err.Error(), "build request: ") {
			t.Fatalf("error = %q, want prefix %q", err.Error(), "build request: ")
		}
		if resp != nil {
			t.Fatal("response != nil, want nil on error")
		}
	})
}

func TestDoDownloadRequest_Headers(t *testing.T) {
	t.Parallel()

	httpc := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodGet {
				t.Fatalf("method = %q, want GET", req.Method)
			}

			if got := req.Header.Get("User-Agent"); got != "test-ua" {
				t.Fatalf("User-Agent = %q, want %q", got, "test-ua")
			}

			if got := req.Header.Get("Accept-Encoding"); got != "identity" {
				t.Fatalf("Accept-Encoding = %q, want identity", got)
			}

			if got := req.Header.Get("Accept"); got != "application/zip, application/octet-stream, */*" {
				t.Fatalf("Accept = %q", got)
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       http.NoBody,
				Header:     make(http.Header),
			}, nil
		}),
	}

	d := download.NewDownloader(&client.Client{
		HTTPClient: httpc,
	})

	resp, err := download.ExportDoDownloadRequest(
		d,
		context.Background(),
		httpc,
		"https://example.com/file.zip",
		"test-ua",
	)
	if err != nil {
		t.Fatalf("DoDownloadRequest() error = %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("response body close error = %v", err)
		}
	}()
}

func TestDoDownloadRequest_RequestError(t *testing.T) {
	t.Parallel()

	errBoom := errors.New("boom")

	httpc := &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errBoom
		}),
	}

	d := download.NewDownloader(&client.Client{
		HTTPClient: httpc,
	})

	resp, err := download.ExportDoDownloadRequest(
		d,
		context.Background(),
		httpc,
		"https://example.com/file.zip",
		"",
	)

	if !errors.Is(err, errBoom) {
		t.Fatalf("error = %v, want %v", err, errBoom)
	}

	if resp != nil {
		t.Fatal("response != nil, want nil")
	}
}

func TestDoDownloadRequest_NilHTTPClient(t *testing.T) {
	t.Parallel()

	d := download.NewDownloader(&client.Client{
		HTTPClient: &http.Client{},
	})

	resp, err := download.ExportDoDownloadRequest(
		d,
		context.Background(),
		nil,
		"https://example.com/file.zip",
		"",
	)

	if err == nil || err.Error() != "download: nil HTTP client" {
		t.Fatalf("error = %v, want nil HTTP client error", err)
	}
	if resp != nil {
		t.Fatalf("response = %#v, want nil", resp)
	}
}

func TestDoDownloadRequest_Redirects(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		target  string
		blocked bool
	}{
		{
			name:   "valid HTTPS redirect",
			target: "https://cdn.example.com/bundle.zip",
		},
		{
			name:    "HTTP redirect",
			target:  "http://cdn.example.com/bundle.zip",
			blocked: true,
		},
		{
			name:    "localhost redirect",
			target:  "https://localhost/bundle.zip",
			blocked: true,
		},
		{
			name:    "localhost with trailing dot",
			target:  "https://localhost./bundle.zip",
			blocked: true,
		},
		{
			name:    "private IPv4 redirect",
			target:  "https://192.168.1.1/bundle.zip",
			blocked: true,
		},
		{
			name:    "loopback IPv6 redirect",
			target:  "https://[::1]/bundle.zip",
			blocked: true,
		},
		{
			name:    "IPv6 zone identifier",
			target:  "https://[::1%25lo]/bundle.zip",
			blocked: true,
		},
		{
			name:    "internal hostname",
			target:  "https://server.internal/bundle.zip",
			blocked: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			calls := 0

			httpc := &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					calls++

					if calls == 1 {
						return &http.Response{
							StatusCode: http.StatusFound,
							Header: http.Header{
								"Location": {tt.target},
							},
							Body: http.NoBody,
						}, nil
					}

					if calls == 2 && req.URL.String() != tt.target {
						t.Errorf(
							"redirect URL = %q, want %q",
							req.URL.String(),
							tt.target,
						)
					}

					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       http.NoBody,
						Request:    req,
					}, nil
				}),
			}

			d := download.NewDownloader(&client.Client{
				HTTPClient: httpc,
			})

			resp, err := download.ExportDoDownloadRequest(
				d,
				context.Background(),
				httpc,
				"https://example.com/file.zip",
				"",
			)

			if tt.blocked {
				if err == nil {
					t.Fatal("expected redirect validation error")
				}
				if !strings.Contains(err.Error(), "invalid redirect") {
					t.Fatalf("error = %v, want invalid redirect", err)
				}
				if resp != nil {
					t.Fatalf("response = %#v, want nil", resp)
				}
				if calls != 1 {
					t.Fatalf("requests = %d, want 1", calls)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			defer func() {
				_ = resp.Body.Close()
			}()

			if calls != 2 {
				t.Fatalf("requests = %d, want 2", calls)
			}
			if got := resp.Request.URL.String(); got != tt.target {
				t.Fatalf("final URL = %q, want %q", got, tt.target)
			}
		})
	}
}

func TestDoDownloadRequest_TooManyRedirects(t *testing.T) {
	t.Parallel()

	calls := 0

	httpc := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			calls++

			return &http.Response{
				StatusCode: http.StatusFound,
				Header: http.Header{
					"Location": {fmt.Sprintf(
						"https://example.com/file.zip?step=%d",
						calls,
					)},
				},
				Body: http.NoBody,
			}, nil
		}),
	}

	d := download.NewDownloader(&client.Client{
		HTTPClient: httpc,
	})

	resp, err := download.ExportDoDownloadRequest(
		d,
		context.Background(),
		httpc,
		"https://example.com/file.zip",
		"",
	)

	if err == nil {
		t.Fatal("expected too many redirects error")
	}
	if !strings.Contains(err.Error(), "too many redirects") {
		t.Fatalf("error = %v, want too many redirects", err)
	}
	if resp != nil {
		t.Fatalf("response = %#v, want nil", resp)
	}
	if calls != 10 {
		t.Fatalf("requests = %d, want 10", calls)
	}
}

func TestDoDownloadRequest_CustomRedirectPolicy(t *testing.T) {
	t.Parallel()

	errBlocked := errors.New("redirect rejected by caller")
	calls := 0
	policyCalls := 0

	httpc := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			calls++

			return &http.Response{
				StatusCode: http.StatusFound,
				Header: http.Header{
					"Location": {"https://cdn.example.com/bundle.zip"},
				},
				Body: http.NoBody,
			}, nil
		}),
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			policyCalls++
			return errBlocked
		},
	}

	d := download.NewDownloader(&client.Client{
		HTTPClient: httpc,
	})

	resp, err := download.ExportDoDownloadRequest(
		d,
		context.Background(),
		httpc,
		"https://example.com/file.zip",
		"",
	)

	if !errors.Is(err, errBlocked) {
		t.Fatalf("error = %v, want %v", err, errBlocked)
	}
	if resp != nil {
		t.Fatalf("response = %#v, want nil", resp)
	}
	if policyCalls != 1 {
		t.Fatalf("policy calls = %d, want 1", policyCalls)
	}
	if calls != 1 {
		t.Fatalf("requests = %d, want 1", calls)
	}
}
