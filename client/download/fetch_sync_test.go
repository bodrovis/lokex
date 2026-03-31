package download_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bodrovis/lokex/v2/client"
	"github.com/bodrovis/lokex/v2/client/download"
	"github.com/bodrovis/lokex/v2/internal/apierr"
	"github.com/jarcoal/httpmock"
)

func TestFetchBundle(t *testing.T) {
	t.Run("nil downloader", func(t *testing.T) {
		t.Parallel()

		var d *download.Downloader

		got, err := d.FetchBundle(context.Background(), strings.NewReader(`{}`))
		if err == nil {
			t.Fatal("FetchBundle() error = nil, want non-nil")
		}
		if err.Error() != "fetch bundle: nil downloader/client" {
			t.Fatalf("error = %q, want %q", err.Error(), "fetch bundle: nil downloader/client")
		}
		if got != "" {
			t.Fatalf("got = %q, want empty string on error", got)
		}
	})

	t.Run("nil client", func(t *testing.T) {
		t.Parallel()

		d := download.ExportNewDownloaderWithClientForTest(nil)

		got, err := d.FetchBundle(context.Background(), strings.NewReader(`{}`))
		if err == nil {
			t.Fatal("FetchBundle() error = nil, want non-nil")
		}
		if err.Error() != "fetch bundle: nil downloader/client" {
			t.Fatalf("error = %q, want %q", err.Error(), "fetch bundle: nil downloader/client")
		}
		if got != "" {
			t.Fatalf("got = %q, want empty string on error", got)
		}
	})

	t.Run("nil context uses background context", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"bundle_url":"https://example.com/file.zip"}`)
		}))
		defer srv.Close()

		d := download.NewDownloader(&client.Client{
			HTTPClient: srv.Client(),
			BaseURL:    srv.URL + "/",
			ProjectID:  "project-id",
		})

		//lint:ignore SA1012 intentionally passing nil context in this test
		got, err := d.FetchBundle(nil, strings.NewReader(`{"format":"json"}`)) //nolint:staticcheck // nil ctx is required for this test
		if err != nil {
			t.Fatalf("FetchBundle() unexpected error = %v", err)
		}
		if got != "https://example.com/file.zip" {
			t.Fatalf("got = %q, want %q", got, "https://example.com/file.zip")
		}
	})

	t.Run("canceled context returns wrapped context error", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		d := download.NewDownloader(&client.Client{
			HTTPClient: &http.Client{},
			ProjectID:  "project-id",
		})

		got, err := d.FetchBundle(ctx, strings.NewReader(`{}`))
		if err == nil {
			t.Fatal("FetchBundle() error = nil, want non-nil")
		}
		if err.Error() != "fetch bundle: context: context canceled" {
			t.Fatalf("error = %q, want %q", err.Error(), "fetch bundle: context: context canceled")
		}
		if got != "" {
			t.Fatalf("got = %q, want empty string on error", got)
		}
	})
}

func TestDownloader_FetchBundle_Variants(t *testing.T) {
	target := fmt.Sprintf("https://api.lokalise.com/api2/projects/%s/files/download", projectID)

	t.Run("happy path with params map", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("POST", target, func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodPost {
				t.Fatalf("method = %s, want POST", req.Method)
			}
			if got := req.Header.Get("X-Api-Token"); got != token {
				t.Fatalf("X-Api-Token = %q, want %q", got, token)
			}
			if got := req.Header.Get("Accept"); got != "application/json" {
				t.Fatalf("Accept = %q, want application/json", got)
			}
			if ct := req.Header.Get("Content-Type"); ct != "application/json" {
				t.Fatalf("Content-Type = %q, want application/json", ct)
			}

			var got map[string]any
			if err := json.NewDecoder(req.Body).Decode(&got); err != nil {
				t.Fatalf("decode req: %v", err)
			}

			if got["format"] != "json" {
				t.Fatalf("format = %v, want json", got["format"])
			}
			tags, ok := got["include_tags"].([]any)
			if !ok || len(tags) != 2 || tags[0] != "one" || tags[1] != "two" {
				t.Fatalf("include_tags wrong: %#v", got["include_tags"])
			}

			return httpmock.NewStringResponse(200, `{"bundle_url":"https://cdn.example.com/bundle.zip"}`), nil
		})

		cli, err := client.NewClient(token, projectID, client.WithHTTPTimeout(3*time.Second))
		if err != nil {
			t.Fatal(err)
		}
		d := download.NewDownloader(cli)

		buf := mustJSONBody(t, map[string]any{
			"include_tags": []string{"one", "two"},
			"format":       "json",
		})

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		url, err := d.FetchBundle(ctx, buf)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if url != "https://cdn.example.com/bundle.zip" {
			t.Fatalf("url=%q, want bundle", url)
		}
	})

	t.Run("empty bundle url", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("POST", target, httpmock.NewStringResponder(200, `{"bundle_url":""}`))

		cli, err := client.NewClient(token, projectID, nil)
		if err != nil {
			t.Fatal(err)
		}
		d := download.NewDownloader(cli)

		buf := mustJSONBody(t, map[string]any{"format": "json"})

		_, err = d.FetchBundle(context.Background(), buf)
		if err == nil || !strings.Contains(err.Error(), "empty bundle url") {
			t.Fatalf("want empty bundle url error, got %v", err)
		}
	})

	t.Run("api non-2xx -> typed apierr", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		respBody := `{"error":{"message":"rate limit","code":429,"details":{"bucket":"global"}}}`
		httpmock.RegisterResponder("POST", target, httpmock.NewStringResponder(429, respBody))

		cli, err := client.NewClient(token, projectID, client.WithBackoff(
			1*time.Millisecond,
			5*time.Millisecond,
		))
		if err != nil {
			t.Fatal(err)
		}

		d := download.NewDownloader(cli)

		buf := mustJSONBody(t, map[string]any{"format": "json"})

		_, err = d.FetchBundle(context.Background(), buf)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		var ae *apierr.APIError
		if !errors.As(err, &ae) {
			t.Fatalf("errors.As failed to find *apierr.APIError: %v", err)
		}
		if ae.Status != http.StatusTooManyRequests || ae.Code != 429 || ae.Message != "rate limit" {
			t.Fatalf("bad apierr: %#v", ae)
		}
	})

	t.Run("success but bad JSON in response -> decode error", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("POST", target, httpmock.NewStringResponder(200, `{"bundle_url":42`)) // broken

		cli, err := client.NewClient(
			token,
			projectID,
		)
		if err != nil {
			t.Fatal(err)
		}

		d := download.NewDownloader(cli)

		buf := mustJSONBody(t, map[string]any{"format": "json"})

		_, err = d.FetchBundle(context.Background(), buf)
		if err == nil || !strings.Contains(err.Error(), "decode response: unexpected end of JSON input") {
			t.Fatalf("want decode response error, got %v", err)
		}
	})

	t.Run("context deadline bubbles up", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("POST", target, func(*http.Request) (*http.Response, error) {
			return nil, context.DeadlineExceeded
		})

		cli, err := client.NewClient(token, projectID, client.WithBackoff(
			1*time.Millisecond,
			5*time.Millisecond,
		))
		if err != nil {
			t.Fatal(err)
		}

		d := download.NewDownloader(cli)

		ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
		defer cancel()

		buf := mustJSONBody(t, map[string]any{"format": "json"})

		_, err = d.FetchBundle(ctx, buf)
		if err == nil || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("want DeadlineExceeded, got %v", err)
		}
	})
}

func TestFetchBundle_EmptyResponseBodyReturnsEmptyBundleURLError(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	target := fmt.Sprintf("https://api.lokalise.com/api2/projects/%s/files/download", projectID)

	// 204 No Content
	httpmock.RegisterResponder("POST", target, func(*http.Request) (*http.Response, error) {
		resp := httpmock.NewStringResponse(204, "")
		return resp, nil
	})

	cli, _ := client.NewClient(token, projectID, nil)
	d := download.NewDownloader(cli)

	buf := mustJSONBody(t, map[string]any{"format": "json"})
	// Expect a decode error about missing bundle_url, not EOF
	_, err := d.FetchBundle(context.Background(), buf)
	if err == nil || !strings.Contains(err.Error(), "empty bundle url") {
		t.Fatalf("want empty bundle url error, got %v", err)
	}
}

func TestDownloader_FetchBundle_NilBody(t *testing.T) {
	cli, _ := client.NewClient(token, projectID, nil)
	d := download.NewDownloader(cli)

	_, err := d.FetchBundle(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "nil request body") {
		t.Fatalf("want nil request body error, got %v", err)
	}
}
