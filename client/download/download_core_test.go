package download_test

import (
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

	"github.com/bodrovis/lokex/v2/client"
	"github.com/bodrovis/lokex/v2/client/download"
	"github.com/bodrovis/lokex/v2/internal/testutils"

	"github.com/jarcoal/httpmock"
)

var (
	token     string
	projectID string
)

func init() {
	if err := testutils.LoadDotEnv(); err != nil {
		log.Printf("warning: could not load .env: %v", err)
	}
	token = testutils.GetEnv("LOKALISE_API_TOKEN", "secret")
	projectID = testutils.GetEnv("LOKALISE_PROJECT_ID", "123.abc")
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
	dl := download.NewDownloader(cli)

	dest := t.TempDir()
	url, err := dl.Download(context.Background(), dest, download.DownloadParams{"format": "json"})
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

func TestDownloader_Download_EmptyUnzipTo(t *testing.T) {
	cli, _ := client.NewClient(token, projectID, nil)
	d := download.NewDownloader(cli)

	_, err := d.Download(context.Background(), "   ", download.DownloadParams{"format": "json"})
	if err == nil || !strings.Contains(err.Error(), "empty unzip destination") {
		t.Fatalf("want empty unzip destination, got %v", err)
	}
}

func TestDownloader_DownloadAsync_FullPipeline(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	// 1) kickoff async
	targetPost := fmt.Sprintf("https://api.lokalise.com/api2/projects/%s/files/async-download", projectID)
	httpmock.RegisterResponder("POST", targetPost,
		httpmock.NewStringResponder(200, `{"process_id":"p1"}`),
	)

	// 2) poll process -> finished + download_url
	targetGet := fmt.Sprintf("https://api.lokalise.com/api2/projects/%s/processes/p1", projectID)
	downloadURL := "https://cdn.example.com/async-full.zip"
	httpmock.RegisterResponder("GET", targetGet, httpmock.NewStringResponder(200, fmt.Sprintf(`{
		"process": {
			"process_id":"p1",
			"status":"finished",
			"details": {"download_url":"%s"}
		}
	}`, downloadURL)))

	// 3) GET zip
	zb := buildZip(t, map[string]string{
		"locales/en/app.json": `{"hello":"async"}`,
		"root.txt":            "ok",
	}, nil)
	registerZipResponder(t, downloadURL, zb)

	cli, _ := client.NewClient(token, projectID, nil)
	d := download.NewDownloader(cli)

	dest := t.TempDir()
	gotURL, err := d.DownloadAsync(context.Background(), dest, download.DownloadParams{"format": "json"})
	if err != nil {
		t.Fatalf("DownloadAsync: %v", err)
	}
	if gotURL != downloadURL {
		t.Fatalf("url=%q, want %q", gotURL, downloadURL)
	}

	// check unzip happened
	b, err := os.ReadFile(filepath.Join(dest, filepath.FromSlash("locales/en/app.json")))
	if err != nil || string(b) != `{"hello":"async"}` {
		t.Fatalf("unzipped file wrong: %v %q", err, string(b))
	}
	b, err = os.ReadFile(filepath.Join(dest, "root.txt"))
	if err != nil || string(b) != "ok" {
		t.Fatalf("unzipped root wrong: %v %q", err, string(b))
	}
}

func TestDownloader_DownloadAsync_EmptyUnzipTo(t *testing.T) {
	cli, _ := client.NewClient(token, projectID, nil)
	d := download.NewDownloader(cli)

	_, err := d.DownloadAsync(context.Background(), "   ", download.DownloadParams{"format": "json"})
	if err == nil || !strings.Contains(err.Error(), "empty unzip destination") {
		t.Fatalf("want empty unzip destination error, got %v", err)
	}
}

func TestDownloader_Download(t *testing.T) {
	t.Parallel()

	t.Run("nil downloader", func(t *testing.T) {
		t.Parallel()

		var d *download.Downloader

		got, err := d.Download(context.Background(), t.TempDir(), download.DownloadParams{"format": "json"})
		if err == nil {
			t.Fatal("Download() error = nil, want non-nil")
		}
		if err.Error() != "download: downloader/client is nil" {
			t.Fatalf("error = %q, want %q", err.Error(), "download: downloader/client is nil")
		}
		if got != "" {
			t.Fatalf("got = %q, want empty string on error", got)
		}
	})

	t.Run("nil client", func(t *testing.T) {
		t.Parallel()

		d := download.ExportNewDownloaderWithClientForTest(nil)

		got, err := d.Download(context.Background(), t.TempDir(), download.DownloadParams{"format": "json"})
		if err == nil {
			t.Fatal("Download() error = nil, want non-nil")
		}
		if err.Error() != "download: downloader/client is nil" {
			t.Fatalf("error = %q, want %q", err.Error(), "download: downloader/client is nil")
		}
		if got != "" {
			t.Fatalf("got = %q, want empty string on error", got)
		}
	})
}

func TestDownloader_DownloadAsync(t *testing.T) {
	t.Parallel()

	t.Run("nil downloader", func(t *testing.T) {
		t.Parallel()

		var d *download.Downloader

		got, err := d.DownloadAsync(context.Background(), t.TempDir(), download.DownloadParams{"format": "json"})
		if err == nil {
			t.Fatal("DownloadAsync() error = nil, want non-nil")
		}
		if err.Error() != "download: downloader/client is nil" {
			t.Fatalf("error = %q, want %q", err.Error(), "download: downloader/client is nil")
		}
		if got != "" {
			t.Fatalf("got = %q, want empty string on error", got)
		}
	})

	t.Run("nil client", func(t *testing.T) {
		t.Parallel()

		d := download.ExportNewDownloaderWithClientForTest(nil)

		got, err := d.DownloadAsync(context.Background(), t.TempDir(), download.DownloadParams{"format": "json"})
		if err == nil {
			t.Fatal("DownloadAsync() error = nil, want non-nil")
		}
		if err.Error() != "download: downloader/client is nil" {
			t.Fatalf("error = %q, want %q", err.Error(), "download: downloader/client is nil")
		}
		if got != "" {
			t.Fatalf("got = %q, want empty string on error", got)
		}
	})
}

func TestDownloader_DoDownload(t *testing.T) {
	t.Run("nil downloader", func(t *testing.T) {
		t.Parallel()

		var d *download.Downloader

		got, err := download.ExportDoDownload(
			d,
			context.Background(),
			t.TempDir(),
			download.DownloadParams{"format": "json"},
			download.ExportNopFetchFunc(),
		)
		if err == nil {
			t.Fatal("DoDownload() error = nil, want non-nil")
		}
		if err.Error() != "download: downloader/client is nil" {
			t.Fatalf("error = %q, want %q", err.Error(), "download: downloader/client is nil")
		}
		if got != "" {
			t.Fatalf("got = %q, want empty string on error", got)
		}
	})

	t.Run("nil client", func(t *testing.T) {
		t.Parallel()

		d := download.ExportNewDownloaderWithClientForTest(nil)

		got, err := download.ExportDoDownload(
			d,
			context.Background(),
			t.TempDir(),
			download.DownloadParams{"format": "json"},
			download.ExportNopFetchFunc(),
		)
		if err == nil {
			t.Fatal("DoDownload() error = nil, want non-nil")
		}
		if err.Error() != "download: downloader/client is nil" {
			t.Fatalf("error = %q, want %q", err.Error(), "download: downloader/client is nil")
		}
		if got != "" {
			t.Fatalf("got = %q, want empty string on error", got)
		}
	})

	t.Run("nil fetch func", func(t *testing.T) {
		t.Parallel()

		cli, err := client.NewClient(token, projectID, nil)
		if err != nil {
			t.Fatal(err)
		}
		d := download.NewDownloader(cli)

		got, err := download.ExportDoDownload(
			d,
			context.Background(),
			t.TempDir(),
			download.DownloadParams{"format": "json"},
			nil,
		)
		if err == nil {
			t.Fatal("DoDownload() error = nil, want non-nil")
		}
		if err.Error() != "download: fetch func is nil" {
			t.Fatalf("error = %q, want %q", err.Error(), "download: fetch func is nil")
		}
		if got != "" {
			t.Fatalf("got = %q, want empty string on error", got)
		}
	})

	t.Run("empty unzipTo", func(t *testing.T) {
		t.Parallel()

		cli, err := client.NewClient(token, projectID, nil)
		if err != nil {
			t.Fatal(err)
		}
		d := download.NewDownloader(cli)

		got, err := download.ExportDoDownload(
			d,
			context.Background(),
			"   \t\n  ",
			download.DownloadParams{"format": "json"},
			download.ExportNopFetchFunc(),
		)
		if err == nil {
			t.Fatal("DoDownload() error = nil, want non-nil")
		}
		if err.Error() != "download: empty unzip destination" {
			t.Fatalf("error = %q, want %q", err.Error(), "download: empty unzip destination")
		}
		if got != "" {
			t.Fatalf("got = %q, want empty string on error", got)
		}
	})

	t.Run("nil context uses background", func(t *testing.T) {
		restore := download.ExportSetDownloadAndUnzipForTest(
			func(_ *download.Downloader, ctx context.Context, bundleURL, destDir string) error {
				if ctx == nil {
					t.Fatal("context = nil, want non-nil")
				}
				if ctx.Err() != nil {
					t.Fatalf("context error = %v, want nil", ctx.Err())
				}
				if bundleURL != "https://example.com/bundle.zip" {
					t.Fatalf("bundleURL = %q, want %q", bundleURL, "https://example.com/bundle.zip")
				}
				if destDir == "" {
					t.Fatal("destDir = empty, want non-empty")
				}
				return nil
			},
		)
		defer restore()

		cli, err := client.NewClient(token, projectID, nil)
		if err != nil {
			t.Fatal(err)
		}
		d := download.NewDownloader(cli)

		fetchCalled := false
		fetch := func(ctx context.Context, body io.Reader) (string, error) {
			fetchCalled = true
			if ctx == nil {
				t.Fatal("fetch context = nil, want non-nil")
			}
			if body == nil {
				t.Fatal("fetch body = nil, want non-nil")
			}
			return "https://example.com/bundle.zip", nil
		}

		got, err := download.ExportDoDownload(
			d,
			nil,
			t.TempDir(),
			download.DownloadParams{"format": "json"},
			fetch,
		)
		if err != nil {
			t.Fatalf("DoDownload() unexpected error = %v", err)
		}
		if !fetchCalled {
			t.Fatal("fetch was not called")
		}
		if got != "https://example.com/bundle.zip" {
			t.Fatalf("got = %q, want %q", got, "https://example.com/bundle.zip")
		}
	})

	t.Run("canceled context is wrapped", func(t *testing.T) {
		t.Parallel()

		cli, err := client.NewClient(token, projectID, nil)
		if err != nil {
			t.Fatal(err)
		}
		d := download.NewDownloader(cli)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		got, err := download.ExportDoDownload(
			d,
			ctx,
			t.TempDir(),
			download.DownloadParams{"format": "json"},
			download.ExportNopFetchFunc(),
		)
		if err == nil {
			t.Fatal("DoDownload() error = nil, want non-nil")
		}
		if err.Error() != "download: context: context canceled" {
			t.Fatalf("error = %q, want %q", err.Error(), "download: context: context canceled")
		}
		if got != "" {
			t.Fatalf("got = %q, want empty string on error", got)
		}
	})

	t.Run("fetch error is returned", func(t *testing.T) {
		t.Parallel()

		cli, err := client.NewClient(token, projectID, nil)
		if err != nil {
			t.Fatal(err)
		}
		d := download.NewDownloader(cli)

		fetch := func(context.Context, io.Reader) (string, error) {
			return "", errors.New("fetch boom")
		}

		got, err := download.ExportDoDownload(
			d,
			context.Background(),
			t.TempDir(),
			download.DownloadParams{"format": "json"},
			fetch,
		)
		if err == nil {
			t.Fatal("DoDownload() error = nil, want non-nil")
		}
		if err.Error() != "fetch boom" {
			t.Fatalf("error = %q, want %q", err.Error(), "fetch boom")
		}
		if got != "" {
			t.Fatalf("got = %q, want empty string on error", got)
		}
	})

	t.Run("DownloadAndUnzip error is returned", func(t *testing.T) {
		restore := download.ExportSetDownloadAndUnzipForTest(
			func(_ *download.Downloader, _ context.Context, bundleURL, destDir string) error {
				if bundleURL != "https://example.com/bundle.zip" {
					t.Fatalf("bundleURL = %q, want %q", bundleURL, "https://example.com/bundle.zip")
				}
				return errors.New("unzip boom")
			},
		)
		defer restore()

		cli, err := client.NewClient(token, projectID, nil)
		if err != nil {
			t.Fatal(err)
		}
		d := download.NewDownloader(cli)

		got, err := download.ExportDoDownload(
			d,
			context.Background(),
			t.TempDir(),
			download.DownloadParams{"format": "json"},
			download.ExportNopFetchFunc(),
		)
		if err == nil {
			t.Fatal("DoDownload() error = nil, want non-nil")
		}
		if err.Error() != "unzip boom" {
			t.Fatalf("error = %q, want %q", err.Error(), "unzip boom")
		}
		if got != "" {
			t.Fatalf("got = %q, want empty string on error", got)
		}
	})

	t.Run("success copies params and returns bundle url", func(t *testing.T) {
		restore := download.ExportSetDownloadAndUnzipForTest(
			func(_ *download.Downloader, _ context.Context, bundleURL, destDir string) error {
				if bundleURL != "https://example.com/final.zip" {
					t.Fatalf("bundleURL = %q, want %q", bundleURL, "https://example.com/final.zip")
				}
				if destDir == "" {
					t.Fatal("destDir = empty, want non-empty")
				}
				return nil
			},
		)
		defer restore()

		cli, err := client.NewClient(token, projectID, nil)
		if err != nil {
			t.Fatal(err)
		}
		d := download.NewDownloader(cli)

		params := download.DownloadParams{
			"format": "json",
			"filter": "all",
		}
		orig := map[string]any{
			"format": params["format"],
			"filter": params["filter"],
		}

		fetch := func(_ context.Context, body io.Reader) (string, error) {
			b, err := io.ReadAll(body)
			if err != nil {
				return "", err
			}
			s := string(b)
			if !strings.Contains(s, `"format":"json"`) {
				t.Fatalf("encoded body = %q, want format field", s)
			}
			if !strings.Contains(s, `"filter":"all"`) {
				t.Fatalf("encoded body = %q, want filter field", s)
			}
			return "https://example.com/final.zip", nil
		}

		got, err := download.ExportDoDownload(
			d,
			context.Background(),
			t.TempDir(),
			params,
			fetch,
		)
		if err != nil {
			t.Fatalf("DoDownload() unexpected error = %v", err)
		}
		if got != "https://example.com/final.zip" {
			t.Fatalf("got = %q, want %q", got, "https://example.com/final.zip")
		}

		if params["format"] != orig["format"] {
			t.Fatalf("params[format] = %v, want %v", params["format"], orig["format"])
		}
		if params["filter"] != orig["filter"] {
			t.Fatalf("params[filter] = %v, want %v", params["filter"], orig["filter"])
		}
	})
}

func TestNewDownloader(t *testing.T) {
	t.Parallel()

	t.Run("nil client panics", func(t *testing.T) {
		t.Parallel()

		defer func() {
			r := recover()
			if r == nil {
				t.Fatal("NewDownloader() panic = nil, want non-nil")
			}
			if r != "lokex/download: nil client passed to NewDownloader" {
				t.Fatalf(
					"panic = %v, want %q",
					r,
					"lokex/download: nil client passed to NewDownloader",
				)
			}
		}()

		_ = download.NewDownloader(nil)
	})
}

func TestDownloader_DoDownload_EmptyParams(t *testing.T) {
	t.Run("nil params uses empty json body", func(t *testing.T) {
		restore := download.ExportSetDownloadAndUnzipForTest(
			func(_ *download.Downloader, _ context.Context, bundleURL, destDir string) error {
				return nil
			},
		)
		defer restore()

		cli, err := client.NewClient(token, projectID, nil)
		if err != nil {
			t.Fatal(err)
		}
		d := download.NewDownloader(cli)

		fetch := func(_ context.Context, body io.Reader) (string, error) {
			b, err := io.ReadAll(body)
			if err != nil {
				return "", err
			}
			if strings.TrimSpace(string(b)) != "{}" {
				t.Fatalf("encoded body = %q, want %q", string(b), "{}")
			}
			return "https://example.com/bundle.zip", nil
		}

		got, err := download.ExportDoDownload(
			d,
			context.Background(),
			t.TempDir(),
			nil,
			fetch,
		)
		if err != nil {
			t.Fatalf("DoDownload() unexpected error = %v", err)
		}
		if got != "https://example.com/bundle.zip" {
			t.Fatalf("got = %q, want %q", got, "https://example.com/bundle.zip")
		}
	})

	t.Run("empty params uses empty json body", func(t *testing.T) {
		restore := download.ExportSetDownloadAndUnzipForTest(
			func(_ *download.Downloader, _ context.Context, bundleURL, destDir string) error {
				return nil
			},
		)
		defer restore()

		cli, err := client.NewClient(token, projectID, nil)
		if err != nil {
			t.Fatal(err)
		}
		d := download.NewDownloader(cli)

		fetch := func(_ context.Context, body io.Reader) (string, error) {
			b, err := io.ReadAll(body)
			if err != nil {
				return "", err
			}
			if strings.TrimSpace(string(b)) != "{}" {
				t.Fatalf("encoded body = %q, want %q", string(b), "{}")
			}
			return "https://example.com/bundle.zip", nil
		}

		got, err := download.ExportDoDownload(
			d,
			context.Background(),
			t.TempDir(),
			download.DownloadParams{},
			fetch,
		)
		if err != nil {
			t.Fatalf("DoDownload() unexpected error = %v", err)
		}
		if got != "https://example.com/bundle.zip" {
			t.Fatalf("got = %q, want %q", got, "https://example.com/bundle.zip")
		}
	})
}

func TestDownloader_DoDownload_EncodeJSONBodyError(t *testing.T) {
	restore := download.ExportSetEncodeJSONBodyForTest(
		func(v any) (*bytes.Reader, error) {
			return nil, errors.New("encode boom")
		},
	)
	defer restore()

	cli, err := client.NewClient(token, projectID, nil)
	if err != nil {
		t.Fatal(err)
	}
	d := download.NewDownloader(cli)

	got, err := download.ExportDoDownload(
		d,
		context.Background(),
		t.TempDir(),
		download.DownloadParams{"format": "json"},
		func(context.Context, io.Reader) (string, error) {
			return "https://example.com/bundle.zip", nil
		},
	)
	if err == nil {
		t.Fatal("DoDownload() error = nil, want non-nil")
	}
	if err.Error() != "download: encode boom" {
		t.Fatalf("error = %q, want %q", err.Error(), "download: encode boom")
	}
	if got != "" {
		t.Fatalf("got = %q, want empty string on error", got)
	}
}
