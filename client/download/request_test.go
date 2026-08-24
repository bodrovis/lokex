package download_test

import (
	"context"
	"errors"
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
	defer resp.Body.Close()
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
