package client_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bodrovis/lokex/apierr"
	"github.com/bodrovis/lokex/client"
	"github.com/bodrovis/lokex/utils"
	"github.com/jarcoal/httpmock"
)

var (
	token     string
	projectID string
)

func init() {
	if err := LoadDotEnv(); err != nil {
		log.Printf("warning: could not load .env: %v", err)
	}
	token = GetEnv("LOKALISE_API_TOKEN", "secret")
	projectID = GetEnv("LOKALISE_PROJECT_ID", "123.abc")
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
		d := client.NewDownloader(cli)

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
		d := client.NewDownloader(cli)

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

		d := client.NewDownloader(cli)

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

		d := client.NewDownloader(cli)

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

		d := client.NewDownloader(cli)

		ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
		defer cancel()

		buf := mustJSONBody(t, map[string]any{"format": "json"})

		_, err = d.FetchBundle(ctx, buf)
		if err == nil || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("want DeadlineExceeded, got %v", err)
		}
	})
}

func TestDownloadAndUnzip_Happy(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	// zip with nested structure
	zb := buildZip(t, map[string]string{
		"locales/en/app.json":     `{"hello":"world"}`,
		"locales/de/app.json":     `{"hallo":"welt"}`,
		"locales/ru/sub/nest.txt": "privet",
		"root.txt":                "top",
	}, nil)

	bundleURL := "https://cdn.example.com/bundle.zip"
	registerZipResponder(t, bundleURL, zb)

	cli, err := client.NewClient(token, projectID, nil)
	if err != nil {
		t.Fatal(err)
	}

	dl := client.NewDownloader(cli)

	dest := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := dl.DownloadAndUnzip(ctx, bundleURL, dest); err != nil {
		t.Fatalf("DownloadAndUnzip: %v", err)
	}

	// assert files exist with content and structure preserved
	checkFile := func(rel, want string) {
		p := filepath.Join(dest, rel)
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		if string(b) != want {
			t.Fatalf("%s content=%q want %q", rel, string(b), want)
		}
	}
	checkFile(filepath.FromSlash("locales/en/app.json"), `{"hello":"world"}`)
	checkFile(filepath.FromSlash("locales/de/app.json"), `{"hallo":"welt"}`)
	checkFile(filepath.FromSlash("locales/ru/sub/nest.txt"), "privet")
	checkFile("root.txt", "top")
}

func TestDownloadAndUnzip_HeadersAreSet(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	zb := buildZip(t, map[string]string{"a.txt": "a"}, nil)
	url := "https://cdn.example.com/h.zip"

	ua := "lokex-tests/1.0"
	registerZipResponderWithHeaderAsserts(t, url, zb, ua)

	cli, err := client.NewClient(token, projectID, client.WithUserAgent(ua))
	if err != nil {
		t.Fatal(err)
	}
	dl := client.NewDownloader(cli)

	dest := t.TempDir()
	if err := dl.DownloadAndUnzip(context.Background(), url, dest); err != nil {
		t.Fatalf("DownloadAndUnzip: %v", err)
	}
}

func TestDownloadAndUnzip_EmptyURL(t *testing.T) {
	cli, _ := client.NewClient(token, projectID, nil)
	dl := client.NewDownloader(cli)

	dest := t.TempDir()
	err := dl.DownloadAndUnzip(context.Background(), "   ", dest)
	if err == nil || !strings.Contains(err.Error(), "empty bundle url") {
		t.Fatalf("want empty bundle url error, got %v", err)
	}
}

func TestDownloader_Download_SyncFlow(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	postURL := fmt.Sprintf("https://api.lokalise.com/api2/projects/%s/files/download", projectID)
	cdnURL := "https://cdn.example.com/sync.zip"

	// POST → bundle_url
	httpmock.RegisterResponder("POST", postURL, func(req *http.Request) (*http.Response, error) {
		var got map[string]any
		_ = json.NewDecoder(req.Body).Decode(&got)
		if got["format"] != "json" {
			t.Fatalf("format = %v, want json", got["format"])
		}
		return httpmock.NewStringResponse(200, `{"bundle_url":"`+cdnURL+`"}`), nil
	})

	// GET ZIP
	zb := buildZip(t, map[string]string{"ok.txt": "ok"}, nil)
	registerZipResponder(t, cdnURL, zb)

	cli, _ := client.NewClient(token, projectID, nil)
	dl := client.NewDownloader(cli)

	dest := t.TempDir()
	url, err := dl.Download(context.Background(), dest, client.DownloadParams{"format": "json"})
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if url != cdnURL {
		t.Fatalf("bundle url mismatch: %q", url)
	}

	b, err := os.ReadFile(filepath.Join(dest, "ok.txt"))
	if err != nil || string(b) != "ok" {
		t.Fatalf("unzipped wrong: %v %q", err, string(b))
	}
}

func TestDownloadAndUnzip_ZipSlipBlocked(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	// entry tries to escape destination
	zb := buildZip(t, map[string]string{
		"../evil.txt": "gotcha",
	}, nil)

	bundleURL := "https://cdn.example.com/evil.zip"
	registerZipResponder(t, bundleURL, zb)

	cli, err := client.NewClient(token, projectID, nil)
	if err != nil {
		t.Fatal(err)
	}

	dl := client.NewDownloader(cli)

	dest := t.TempDir()
	err = dl.DownloadAndUnzip(context.Background(), bundleURL, dest)
	if err == nil || !strings.Contains(err.Error(), "unsafe path") {
		t.Fatalf("want unsafe path error, got %v", err)
	}
	// ensure it didn't create evil.txt outside; we can't easily check outside,
	// but we can ensure it didn't place anything inside dest either.
	entries, _ := os.ReadDir(dest)
	if len(entries) != 0 {
		t.Fatalf("dest not empty after blocked zip-slip: %+v", entries)
	}
}

func TestDownloadAndUnzip_SymlinkEntry_Skipped(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	// include a regular file + a symlink entry
	zb := buildZip(t, map[string]string{
		"dir/file.txt": "data",
	}, map[string]string{
		"dir/link": "file.txt",
	})

	bundleURL := "https://cdn.example.com/with-symlink.zip"
	registerZipResponder(t, bundleURL, zb)

	cli, err := client.NewClient(token, projectID, nil)
	if err != nil {
		t.Fatal(err)
	}

	dl := client.NewDownloader(cli)

	dest := t.TempDir()
	if err := dl.DownloadAndUnzip(context.Background(), bundleURL, dest); err != nil {
		t.Fatalf("DownloadAndUnzip: %v", err)
	}

	// file exists
	content, err := os.ReadFile(filepath.Join(dest, "dir", "file.txt"))
	if err != nil || string(content) != "data" {
		t.Fatalf("file content wrong: %v %q", err, string(content))
	}

	// symlink strategy: in our impl we skip symlinks; confirm it doesn't exist or is not a regular file
	linkPath := filepath.Join(dest, "dir", "link")
	if info, err := os.Lstat(linkPath); err == nil {
		// If platform allows symlink creation without admin (unlikely on Windows), it might exist.
		// Accept either a symlink or skipped (non-existent). But it must NOT be a regular file with copied content.
		if info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("expected symlink or nothing at %s, got non-symlink: mode=%v", linkPath, info.Mode())
		}
	}
}

func TestDownloadAndUnzip_HTTPNon2xx(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	url := "https://cdn.example.com/notfound.zip"
	httpmock.RegisterResponder("GET", url, httpmock.NewStringResponder(404, "nope"))

	cli, err := client.NewClient(token, projectID, client.WithBackoff(
		1*time.Millisecond,
		5*time.Millisecond,
	))
	if err != nil {
		t.Fatal(err)
	}

	dl := client.NewDownloader(cli)

	dest := t.TempDir()
	err = dl.DownloadAndUnzip(context.Background(), url, dest)
	if err == nil {
		t.Fatal("want error, got nil")
	}

	// we return *apierr.APIError and should NOT retry on 4xx like 404
	var ae *apierr.APIError
	if !errors.As(err, &ae) {
		t.Fatalf("errors.As failed to find *apierr.APIError: %v", err)
	}
	if ae.Status != http.StatusNotFound {
		t.Fatalf("status=%d, want 404", ae.Status)
	}

	info := httpmock.GetCallCountInfo()
	if got := info["GET "+url]; got != 1 {
		t.Fatalf("should not retry on 404, got %d attempts", got)
	}
}

func TestDownloadAndUnzip_RetryOn5xxThenSuccess(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	// build a valid zip
	zb := buildZip(t, map[string]string{
		"ok.txt": "ok",
	}, nil)

	url := "https://cdn.example.com/flaky.zip"
	attempt := 0
	httpmock.RegisterResponder("GET", url, func(*http.Request) (*http.Response, error) {
		attempt++
		if attempt <= 2 {
			return httpmock.NewStringResponse(503, "try later"), nil
		}
		return httpmock.NewBytesResponse(200, zb), nil
	})

	cli, err := client.NewClient(token, projectID, client.WithBackoff(
		1*time.Millisecond,
		5*time.Millisecond,
	))
	if err != nil {
		t.Fatal(err)
	}

	dl := client.NewDownloader(cli)

	dest := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := dl.DownloadAndUnzip(ctx, url, dest); err != nil {
		t.Fatalf("DownloadAndUnzip: %v", err)
	}

	// succeeded on 3rd attempt (2x 503 + 1x 200)
	info := httpmock.GetCallCountInfo()
	if got := info["GET "+url]; got != 3 {
		t.Fatalf("attempts=%d, want 3", got)
	}

	// file landed
	b, err := os.ReadFile(filepath.Join(dest, "ok.txt"))
	if err != nil || string(b) != "ok" {
		t.Fatalf("unzipped file wrong: %v %q", err, string(b))
	}
}

func TestDownloadAndUnzip_RetryStopsAtMax(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	url := "https://cdn.example.com/down.zip"
	httpmock.RegisterResponder("GET", url, httpmock.NewStringResponder(500, "boom"))

	cli, err := client.NewClient(token, projectID, client.WithBackoff(
		1*time.Millisecond,
		5*time.Millisecond,
	))
	if err != nil {
		t.Fatal(err)
	}

	dl := client.NewDownloader(cli)

	dest := t.TempDir()
	err = dl.DownloadAndUnzip(context.Background(), url, dest)
	if err == nil {
		t.Fatal("want error, got nil")
	}

	// should be *apierr.APIError with 500
	var ae *apierr.APIError
	if !errors.As(err, &ae) {
		t.Fatalf("errors.As failed to find *apierr.APIError: %v", err)
	}
	if ae.Status != http.StatusInternalServerError {
		t.Fatalf("status=%d, want 500", ae.Status)
	}

	// attempts = maxRetries + 1 = 4
	info := httpmock.GetCallCountInfo()
	if got := info["GET "+url]; got != 4 {
		t.Fatalf("attempts=%d, want 4", got)
	}
}

func TestDownloadAndUnzip_BackoffCanceledByContext(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	url := "https://cdn.example.com/flaky-cancel.zip"
	httpmock.RegisterResponder("GET", url, httpmock.NewStringResponder(500, "boom"))

	cli, err := client.NewClient(token, projectID, client.WithBackoff(
		50*time.Millisecond,
		100*time.Millisecond,
	))
	if err != nil {
		t.Fatal(err)
	}

	dl := client.NewDownloader(cli)

	dest := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	// cancel shortly after the first failing attempt; should exit during backoff sleep
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	err = dl.DownloadAndUnzip(ctx, url, dest)
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("want context canceled, got %v", err)
	}

	// since we canceled during backoff, only the first request should have happened
	info := httpmock.GetCallCountInfo()
	if got := info["GET "+url]; got != 1 {
		t.Fatalf("attempts=%d, want 1", got)
	}
}

func TestDownloadAndUnzip_ContextCanceled(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	url := "https://cdn.example.com/slow.zip"
	httpmock.RegisterResponder("GET", url, func(*http.Request) (*http.Response, error) {
		return nil, context.Canceled
	})

	cli, err := client.NewClient(token, projectID, client.WithBackoff(
		1*time.Millisecond,
		5*time.Millisecond,
	))
	if err != nil {
		t.Fatal(err)
	}

	dl := client.NewDownloader(cli)

	dest := t.TempDir()
	err = dl.DownloadAndUnzip(context.Background(), url, dest)
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("want context canceled, got %v", err)
	}
}

func TestIntegration_Download(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in -short mode")
	}

	cli, err := client.NewClient(token, projectID, nil)
	if err != nil {
		t.Fatal(err)
	}

	d := client.NewDownloader(cli)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	localesDir := filepath.Join("./", "locales")

	url, err := d.Download(ctx, localesDir, client.DownloadParams{
		"format": "json",
	})
	if err != nil {
		t.Fatalf("integration request failed: %v", err)
	}
	if !strings.HasPrefix(url, "https://") {
		t.Fatalf("unexpected bundle URL: %q", url)
	}

	// 1) locales exists and is not empty
	entries, err := os.ReadDir(localesDir)
	if err != nil {
		t.Fatalf("cannot read locales dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("locales dir is empty")
	}

	// 2) recursively ensure at least one regular file exists
	foundFile := false
	err = filepath.WalkDir(localesDir, func(path string, d os.DirEntry, _ error) error {
		if !d.IsDir() {
			foundFile = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk locales dir: %v", err)
	}
	if !foundFile {
		t.Fatalf("no files found under locales dir (only directories present)")
	}
}

func TestIntegration_DownloadAsync(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in -short mode")
	}

	cli, err := client.NewClient(token, projectID, nil)
	if err != nil {
		t.Fatal(err)
	}
	d := client.NewDownloader(cli)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	localesDir := filepath.Join("./", "locales-async")

	url, err := d.DownloadAsync(ctx, localesDir, client.DownloadParams{
		"format": "json",
	})
	if err != nil {
		t.Fatalf("integration request failed: %v", err)
	}
	if !strings.HasPrefix(url, "https://") {
		t.Fatalf("unexpected bundle URL: %q", url)
	}

	// 1) locales exists and is not empty
	entries, err := os.ReadDir(localesDir)
	if err != nil {
		t.Fatalf("cannot read locales dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("locales dir is empty")
	}

	// 2) recursively ensure at least one regular file exists
	foundFile := false
	err = filepath.WalkDir(localesDir, func(path string, d os.DirEntry, _ error) error {
		if !d.IsDir() {
			foundFile = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk locales dir: %v", err)
	}
	if !foundFile {
		t.Fatalf("no files found under locales dir (only directories present)")
	}
}

func TestDownloader_FetchBundleAsync(t *testing.T) {
	targetPost := fmt.Sprintf("https://api.lokalise.com/api2/projects/%s/files/async-download", projectID)
	targetGet := fmt.Sprintf("https://api.lokalise.com/api2/projects/%s/processes/xyz", projectID)

	t.Run("happy path async", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// async kickoff
		httpmock.RegisterResponder("POST", targetPost,
			httpmock.NewStringResponder(200, `{"process_id":"xyz"}`))

		// process poller: return finished with download_url
		httpmock.RegisterResponder("GET", targetGet, httpmock.NewStringResponder(200, `{
			"process": {
				"process_id":"xyz",
				"status":"finished",
				"details": {"download_url":"https://cdn.example.com/async-bundle.zip"}
			}
		}`))

		cli, err := client.NewClient(token, projectID, nil)
		if err != nil {
			t.Fatal(err)
		}

		d := client.NewDownloader(cli)

		buf := mustJSONBody(t, map[string]any{"format": "json"})

		url, err := d.FetchBundleAsync(context.Background(), buf)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if url != "https://cdn.example.com/async-bundle.zip" {
			t.Fatalf("url=%q, want bundle url", url)
		}
	})

	t.Run("failed process", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("POST", targetPost,
			httpmock.NewStringResponder(200, `{"process_id":"xyz"}`))

		// first poll -> failed
		httpmock.RegisterResponder("GET", targetGet, httpmock.NewStringResponder(200, `{
			"process": {"process_id":"xyz","status":"failed"}
		}`))

		cli, err := client.NewClient(token, projectID, nil)
		if err != nil {
			t.Fatal(err)
		}

		d := client.NewDownloader(cli)

		buf := mustJSONBody(t, map[string]any{"format": "json"})

		_, err = d.FetchBundleAsync(context.Background(), buf)
		if err == nil || !strings.Contains(err.Error(), "failed") {
			t.Fatalf("want failed error, got %v", err)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("POST", targetPost,
			httpmock.NewStringResponder(200, `{"process_id":"xyz"}`))

		// always queued, never finishes
		httpmock.RegisterResponder("GET", targetGet, httpmock.NewStringResponder(200, `{
			"process": {"process_id":"xyz","status":"queued"}
		}`))

		cli, err := client.NewClient(token, projectID, nil)
		if err != nil {
			t.Fatal(err)
		}

		d := client.NewDownloader(cli)

		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		buf := mustJSONBody(t, map[string]any{"format": "json"})

		_, err = d.FetchBundleAsync(ctx, buf)
		if err == nil || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("want context deadline, got %v", err)
		}
	})
}

func TestDownloadAndUnzip_RetryOnUnexpectedEOFThenSuccess(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	// valid zip for the second attempt
	okZip := buildZip(t, map[string]string{"ok.txt": "ok"}, nil)

	url := "https://cdn.example.com/trunc-then-ok.zip"
	attempt := 0
	httpmock.RegisterResponder("GET", url, func(_ *http.Request) (*http.Response, error) {
		attempt++
		if attempt == 1 {
			// body shorter than Content-Length => triggers our short-read check
			resp := httpmock.NewBytesResponse(200, []byte("short"))
			resp.Header.Set("Content-Length", "12345")
			resp.Header.Set("Content-Type", "application/zip")
			return resp, nil
		}
		return httpmock.NewBytesResponse(200, okZip), nil
	})

	cli, _ := client.NewClient(token, projectID, client.WithBackoff(1*time.Millisecond, 5*time.Millisecond))
	dl := client.NewDownloader(cli)

	dest := t.TempDir()
	if err := dl.DownloadAndUnzip(context.Background(), url, dest); err != nil {
		t.Fatalf("DownloadAndUnzip: %v", err)
	}

	// retried exactly once
	if got := httpmock.GetCallCountInfo()["GET "+url]; got != 2 {
		t.Fatalf("attempts=%d, want 2", got)
	}
	if b, _ := os.ReadFile(filepath.Join(dest, "ok.txt")); string(b) != "ok" {
		t.Fatalf("unzipped wrong/missing")
	}
}

func TestDownloadAndUnzip_RetryOnCorruptZipThenSuccess(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	okZip := buildZip(t, map[string]string{"x.txt": "x"}, nil)

	url := "https://cdn.example.com/corrupt-then-ok.zip"
	attempt := 0
	httpmock.RegisterResponder("GET", url, func(_ *http.Request) (*http.Response, error) {
		attempt++
		if attempt == 1 {
			return httpmock.NewBytesResponse(200, []byte("not a zip")), nil
		}
		return httpmock.NewBytesResponse(200, okZip), nil
	})

	cli, _ := client.NewClient(token, projectID, client.WithBackoff(1*time.Millisecond, 5*time.Millisecond))
	dl := client.NewDownloader(cli)

	dest := t.TempDir()
	if err := dl.DownloadAndUnzip(context.Background(), url, dest); err != nil {
		t.Fatalf("DownloadAndUnzip: %v", err)
	}
	if got := httpmock.GetCallCountInfo()["GET "+url]; got != 2 {
		t.Fatalf("attempts=%d, want 2", got)
	}
}

func TestFetcher_AllowsEmptyBodyOn2xx(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	target := fmt.Sprintf("https://api.lokalise.com/api2/projects/%s/files/download", projectID)

	// 204 No Content
	httpmock.RegisterResponder("POST", target, func(*http.Request) (*http.Response, error) {
		resp := httpmock.NewStringResponse(204, "")
		return resp, nil
	})

	cli, _ := client.NewClient(token, projectID, nil)
	d := client.NewDownloader(cli)

	buf := mustJSONBody(t, map[string]any{"format": "json"})
	// Expect a decode error about missing bundle_url, not EOF
	_, err := d.FetchBundle(context.Background(), buf)
	if err == nil || !strings.Contains(err.Error(), "empty bundle url") {
		t.Fatalf("want empty bundle url error, got %v", err)
	}
}

func buildZip(t *testing.T, entries map[string]string, symlinks map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// regular files
	for name, content := range entries {
		fh := &zip.FileHeader{
			Name:   name,
			Method: zip.Deflate,
		}
		// ensure dirs implied by name are created in archive entries (zip doesn't need explicit dir entries)
		fh.SetMode(0o644)
		w, err := zw.CreateHeader(fh)
		if err != nil {
			t.Fatalf("CreateHeader(%s): %v", name, err)
		}
		if _, err := io.Copy(w, strings.NewReader(content)); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	// symlinks (write target path as file content; unzipSafe should skip or handle)
	for link, target := range symlinks {
		fh := &zip.FileHeader{
			Name:   link,
			Method: zip.Store,
		}
		// mark as symlink
		fh.SetMode(os.ModeSymlink | 0o777)
		w, err := zw.CreateHeader(fh)
		if err != nil {
			t.Fatalf("CreateHeader(symlink %s): %v", link, err)
		}
		if _, err := io.Copy(w, strings.NewReader(target)); err != nil {
			t.Fatalf("write symlink %s: %v", link, err)
		}
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

func registerZipResponder(t *testing.T, url string, zipBytes []byte) {
	t.Helper()
	httpmock.RegisterResponder("GET", url, func(req *http.Request) (*http.Response, error) {
		// we could assert UA here if needed
		return httpmock.NewBytesResponse(200, zipBytes), nil
	})
}

func registerZipResponderWithHeaderAsserts(t *testing.T, url string, zipBytes []byte, wantUA string) {
	t.Helper()
	httpmock.RegisterResponder("GET", url, func(req *http.Request) (*http.Response, error) {
		if got := req.Header.Get("User-Agent"); wantUA != "" && got != wantUA {
			t.Fatalf("GET UA = %q, want %q", got, wantUA)
		}
		if got := req.Header.Get("Accept"); got == "" {
			t.Fatalf("GET Accept header missing")
		}
		if got := req.Header.Get("Accept-Encoding"); got != "identity" {
			t.Fatalf("GET Accept-Encoding = %q, want identity", got)
		}
		return httpmock.NewBytesResponse(200, zipBytes), nil
	})
}

func mustJSONBody(t *testing.T, m map[string]any) io.Reader {
	t.Helper()
	r, err := utils.EncodeJSONBody(m)
	if err != nil {
		t.Fatalf("encode body: %v", err)
	}
	return r
}
