package download_test

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bodrovis/lokex/v2/client"
	"github.com/bodrovis/lokex/v2/client/download"
	"github.com/bodrovis/lokex/v2/internal/apierr"
	"github.com/jarcoal/httpmock"
)

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

	dl := download.NewDownloader(cli)

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
	dl := download.NewDownloader(cli)

	dest := t.TempDir()
	if err := dl.DownloadAndUnzip(context.Background(), url, dest); err != nil {
		t.Fatalf("DownloadAndUnzip: %v", err)
	}
}

func TestDownloadAndUnzip_EmptyURL(t *testing.T) {
	cli, _ := client.NewClient(token, projectID, nil)
	dl := download.NewDownloader(cli)

	dest := t.TempDir()
	err := dl.DownloadAndUnzip(context.Background(), "   ", dest)
	if err == nil || !strings.Contains(err.Error(), "empty bundle url") {
		t.Fatalf("want empty bundle url error, got %v", err)
	}
}

func TestDownloadAndUnzip_InvalidURL(t *testing.T) {
	cli, _ := client.NewClient(token, projectID, nil)
	dl := download.NewDownloader(cli)

	dest := t.TempDir()
	err := dl.DownloadAndUnzip(context.Background(), "localhost", dest)
	if err == nil || !strings.Contains(err.Error(), "unsupported url scheme") {
		t.Fatalf("want empty bundle url error, got %v", err)
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

	dl := download.NewDownloader(cli)

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

	dl := download.NewDownloader(cli)

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

	dl := download.NewDownloader(cli)

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

	dl := download.NewDownloader(cli)

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

	dl := download.NewDownloader(cli)

	dest := t.TempDir()
	err = dl.DownloadAndUnzip(context.Background(), url, dest)
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("want context canceled, got %v", err)
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

	dl := download.NewDownloader(cli)

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

	dl := download.NewDownloader(cli)

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
	dl := download.NewDownloader(cli)

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
	dl := download.NewDownloader(cli)

	dest := t.TempDir()
	if err := dl.DownloadAndUnzip(context.Background(), url, dest); err != nil {
		t.Fatalf("DownloadAndUnzip: %v", err)
	}
	if got := httpmock.GetCallCountInfo()["GET "+url]; got != 2 {
		t.Fatalf("attempts=%d, want 2", got)
	}
}

func TestDownloadAndUnzip_EmptyDestDir(t *testing.T) {
	cli, _ := client.NewClient(token, projectID, nil)
	dl := download.NewDownloader(cli)

	err := dl.DownloadAndUnzip(context.Background(), "https://cdn.example.com/bundle.zip", "   ")
	if err == nil || !strings.Contains(err.Error(), "empty dest dir") {
		t.Fatalf("want empty dest dir error, got %v", err)
	}
}

func TestDownloadAndUnzip_ContextAlreadyCanceled_NoRequest(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	url := "https://cdn.example.com/never-called.zip"

	cli, _ := client.NewClient(token, projectID, nil)
	dl := download.NewDownloader(cli)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	dest := t.TempDir()
	err := dl.DownloadAndUnzip(ctx, url, dest)
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}

	if got := httpmock.GetCallCountInfo()["GET "+url]; got != 0 {
		t.Fatalf("GET attempts = %d, want 0", got)
	}
}

func TestDownloadAndUnzipPrecheck(t *testing.T) {
	t.Run("nil downloader", func(t *testing.T) {
		t.Parallel()

		var d *download.Downloader

		gotCtx, gotURL, gotDest, err := download.ExportDownloadAndUnzipPrecheck(
			d,
			context.Background(),
			"https://example.com/bundle.zip",
			t.TempDir(),
		)
		if err == nil {
			t.Fatal("DownloadAndUnzipPrecheck() error = nil, want non-nil")
		}
		if err.Error() != "download: downloader/client/http client is nil" {
			t.Fatalf("error = %q, want %q", err.Error(), "download: downloader/client/http client is nil")
		}
		if gotCtx != nil {
			t.Fatal("context != nil, want nil on error")
		}
		if gotURL != "" {
			t.Fatalf("url = %q, want empty string on error", gotURL)
		}
		if gotDest != "" {
			t.Fatalf("dest = %q, want empty string on error", gotDest)
		}
	})

	t.Run("nil client", func(t *testing.T) {
		t.Parallel()

		d := download.ExportNewDownloaderWithClientForTest(nil)

		gotCtx, gotURL, gotDest, err := download.ExportDownloadAndUnzipPrecheck(
			d,
			context.Background(),
			"https://example.com/bundle.zip",
			t.TempDir(),
		)
		if err == nil {
			t.Fatal("DownloadAndUnzipPrecheck() error = nil, want non-nil")
		}
		if err.Error() != "download: downloader/client/http client is nil" {
			t.Fatalf("error = %q, want %q", err.Error(), "download: downloader/client/http client is nil")
		}
		if gotCtx != nil {
			t.Fatal("context != nil, want nil on error")
		}
		if gotURL != "" {
			t.Fatalf("url = %q, want empty string on error", gotURL)
		}
		if gotDest != "" {
			t.Fatalf("dest = %q, want empty string on error", gotDest)
		}
	})

	t.Run("nil http client", func(t *testing.T) {
		t.Parallel()

		d := download.ExportNewDownloaderWithClientForTest(&client.Client{
			HTTPClient: nil,
			ProjectID:  projectID,
		})

		gotCtx, gotURL, gotDest, err := download.ExportDownloadAndUnzipPrecheck(
			d,
			context.Background(),
			"https://example.com/bundle.zip",
			t.TempDir(),
		)
		if err == nil {
			t.Fatal("DownloadAndUnzipPrecheck() error = nil, want non-nil")
		}
		if err.Error() != "download: downloader/client/http client is nil" {
			t.Fatalf("error = %q, want %q", err.Error(), "download: downloader/client/http client is nil")
		}
		if gotCtx != nil {
			t.Fatal("context != nil, want nil on error")
		}
		if gotURL != "" {
			t.Fatalf("url = %q, want empty string on error", gotURL)
		}
		if gotDest != "" {
			t.Fatalf("dest = %q, want empty string on error", gotDest)
		}
	})

	t.Run("nil context uses background", func(t *testing.T) {
		t.Parallel()

		cli, err := client.NewClient(token, projectID, nil)
		if err != nil {
			t.Fatal(err)
		}

		d := download.NewDownloader(cli)
		destDir := t.TempDir()

		gotCtx, gotURL, gotDest, err := download.ExportDownloadAndUnzipPrecheck(
			d,
			nil,
			"  https://example.com/bundle.zip  ",
			"  "+destDir+"  ",
		)
		if err != nil {
			t.Fatalf("DownloadAndUnzipPrecheck() unexpected error = %v", err)
		}
		if gotCtx == nil {
			t.Fatal("context = nil, want non-nil")
		}
		if gotCtx.Err() != nil {
			t.Fatalf("context error = %v, want nil", gotCtx.Err())
		}
		if gotURL != "https://example.com/bundle.zip" {
			t.Fatalf("url = %q, want %q", gotURL, "https://example.com/bundle.zip")
		}
		if gotDest != destDir {
			t.Fatalf("dest = %q, want %q", gotDest, destDir)
		}
	})
}

func TestEnsureDestDir(t *testing.T) {
	t.Run("mkdir all error is wrapped", func(t *testing.T) {
		restore := download.ExportSetMkdirAllForTest(func(path string, perm os.FileMode) error {
			return errors.New("mkdir boom")
		})
		defer restore()

		err := download.ExportEnsureDestDir("/nope")
		if err == nil {
			t.Fatal("EnsureDestDir() error = nil, want non-nil")
		}
		if err.Error() != "download: create dest: mkdir boom" {
			t.Fatalf("error = %q, want %q", err.Error(), "download: create dest: mkdir boom")
		}
	})
}

func TestCreateDownloadTempDir(t *testing.T) {
	t.Run("mkdir temp error is wrapped", func(t *testing.T) {
		restore := download.ExportSetMkdirTempForTest(func(dir, pattern string) (string, error) {
			return "", errors.New("mktemp boom")
		})
		defer restore()

		tmpDir, cleanup, err := download.ExportCreateDownloadTempDir()
		if err == nil {
			t.Fatal("CreateDownloadTempDir() error = nil, want non-nil")
		}
		if err.Error() != "download: create temp dir: mktemp boom" {
			t.Fatalf("error = %q, want %q", err.Error(), "download: create temp dir: mktemp boom")
		}
		if tmpDir != "" {
			t.Fatalf("tmpDir = %q, want empty string on error", tmpDir)
		}
		if cleanup != nil {
			t.Fatal("cleanup != nil, want nil on error")
		}
	})
}

func TestDownloadAndUnzip(t *testing.T) {
	t.Run("ensure dest dir error is returned", func(t *testing.T) {
		restore := download.ExportSetMkdirAllForTest(func(path string, perm os.FileMode) error {
			return errors.New("mkdir boom")
		})
		defer restore()

		cli, err := client.NewClient(token, projectID, nil)
		if err != nil {
			t.Fatal(err)
		}

		d := download.NewDownloader(cli)

		err = d.DownloadAndUnzip(context.Background(), "https://example.com/bundle.zip", t.TempDir())
		if err == nil {
			t.Fatal("DownloadAndUnzip() error = nil, want non-nil")
		}
		if err.Error() != "download: create dest: mkdir boom" {
			t.Fatalf("error = %q, want %q", err.Error(), "download: create dest: mkdir boom")
		}
	})

	t.Run("create temp dir error is returned", func(t *testing.T) {
		restore := download.ExportSetMkdirTempForTest(func(dir, pattern string) (string, error) {
			return "", errors.New("mktemp boom")
		})
		defer restore()

		cli, err := client.NewClient(token, projectID, nil)
		if err != nil {
			t.Fatal(err)
		}

		d := download.NewDownloader(cli)

		err = d.DownloadAndUnzip(context.Background(), "https://example.com/bundle.zip", t.TempDir())
		if err == nil {
			t.Fatal("DownloadAndUnzip() error = nil, want non-nil")
		}
		if err.Error() != "download: create temp dir: mktemp boom" {
			t.Fatalf("error = %q, want %q", err.Error(), "download: create temp dir: mktemp boom")
		}
	})
}
