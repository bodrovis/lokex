package download_test

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/bodrovis/lokex/v2/client"
	"github.com/bodrovis/lokex/v2/client/download"
)

func TestDownloadOncePrecheck(t *testing.T) {
	t.Parallel()

	mustClient := func(httpc *http.Client) *client.Client {
		return &client.Client{
			HTTPClient: httpc,
		}
	}

	tests := []struct {
		name         string
		downloader   *download.Downloader
		ctx          context.Context
		urlStr       string
		destPath     string
		wantURL      string
		wantDestPath string
		wantErr      string
	}{
		{
			name:       "nil downloader",
			downloader: nil,
			ctx:        context.Background(),
			urlStr:     "https://example.com/file.zip",
			destPath:   "/tmp/file.zip",
			wantErr:    "download: downloader/client/http client is nil",
		},
		{
			name: "nil http client in client",
			downloader: download.NewDownloader(&client.Client{
				HTTPClient: nil,
			}),
			ctx:      context.Background(),
			urlStr:   "https://example.com/file.zip",
			destPath: "/tmp/file.zip",
			wantErr:  "download: downloader/client/http client is nil",
		},
		{
			name:       "nil context",
			downloader: download.NewDownloader(mustClient(&http.Client{})),
			ctx:        nil,
			urlStr:     "https://example.com/file.zip",
			destPath:   "/tmp/file.zip",
			wantErr:    "download: nil context",
		},
		{
			name:       "canceled context",
			downloader: download.NewDownloader(mustClient(&http.Client{})),
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			}(),
			urlStr:   "https://example.com/file.zip",
			destPath: "/tmp/file.zip",
			wantErr:  "context canceled",
		},
		{
			name:       "empty url after trim",
			downloader: download.NewDownloader(mustClient(&http.Client{})),
			ctx:        context.Background(),
			urlStr:     "   \t\n  ",
			destPath:   "/tmp/file.zip",
			wantErr:    "download: empty url",
		},
		{
			name:       "empty dest path after trim",
			downloader: download.NewDownloader(mustClient(&http.Client{})),
			ctx:        context.Background(),
			urlStr:     "https://example.com/file.zip",
			destPath:   "   \t\n  ",
			wantErr:    "download: empty dest path",
		},
		{
			name:         "success trims inputs",
			downloader:   download.NewDownloader(mustClient(&http.Client{})),
			ctx:          context.Background(),
			urlStr:       "  https://example.com/file.zip  ",
			destPath:     "  /tmp/file.zip  ",
			wantURL:      "https://example.com/file.zip",
			wantDestPath: "/tmp/file.zip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			httpc, gotURL, gotDestPath, err := download.ExportDownloadOncePrecheck(
				tt.downloader,
				tt.ctx,
				tt.urlStr,
				tt.destPath,
			)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("DownloadOncePrecheck() error = nil, want %q", tt.wantErr)
				}
				if err.Error() != tt.wantErr {
					t.Fatalf("error = %q, want %q", err.Error(), tt.wantErr)
				}
				if httpc != nil {
					t.Fatal("http client != nil on error, want nil")
				}
				if gotURL != "" {
					t.Fatalf("url = %q, want empty string on error", gotURL)
				}
				if gotDestPath != "" {
					t.Fatalf("dest path = %q, want empty string on error", gotDestPath)
				}
				return
			}

			if err != nil {
				t.Fatalf("DownloadOncePrecheck() unexpected error = %v", err)
			}
			if httpc == nil {
				t.Fatal("http client = nil, want non-nil")
			}
			if gotURL != tt.wantURL {
				t.Fatalf("url = %q, want %q", gotURL, tt.wantURL)
			}
			if gotDestPath != tt.wantDestPath {
				t.Fatalf("dest path = %q, want %q", gotDestPath, tt.wantDestPath)
			}
		})
	}
}

func TestDownloadOnce_PrecheckErrorIsReturned(t *testing.T) {
	t.Parallel()

	d := download.NewDownloader(&client.Client{
		HTTPClient: &http.Client{},
	})

	err := download.ExportDownloadOnce(
		d,
		context.Background(),
		"   ",
		filepath.Join(t.TempDir(), "bundle.zip"),
		"test-ua",
	)
	if err == nil {
		t.Fatal("DownloadOnce() error = nil, want non-nil")
	}
	if err.Error() != "download: empty url" {
		t.Fatalf("error = %q, want %q", err.Error(), "download: empty url")
	}
}

func TestDownloadOnce(t *testing.T) {
	t.Run("do download request error is returned", func(t *testing.T) {
		restore := download.ExportSetDoDownloadRequestForTest(
			func(
				_ *download.Downloader,
				_ context.Context,
				_ *http.Client,
				_ string,
				_ string,
			) (*http.Response, error) {
				return nil, errors.New("request boom")
			},
		)
		defer restore()

		d := download.NewDownloader(&client.Client{
			HTTPClient: &http.Client{},
		})

		destPath := filepath.Join(t.TempDir(), "bundle.zip")

		err := download.ExportDownloadOnce(
			d,
			context.Background(),
			"https://example.com/file.zip",
			destPath,
			"test-ua",
		)
		if err == nil {
			t.Fatal("DownloadOnce() error = nil, want non-nil")
		}
		if err.Error() != "request boom" {
			t.Fatalf("error = %q, want %q", err.Error(), "request boom")
		}
	})
}
