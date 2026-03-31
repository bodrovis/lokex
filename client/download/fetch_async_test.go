package download_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bodrovis/lokex/v2/client"
	"github.com/bodrovis/lokex/v2/client/download"
	"github.com/bodrovis/lokex/v2/client/internal/background"
	"github.com/jarcoal/httpmock"
)

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

		d := download.NewDownloader(cli)

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

		d := download.NewDownloader(cli)

		buf := mustJSONBody(t, map[string]any{"format": "json"})

		_, err = d.FetchBundleAsync(context.Background(), buf)
		if err == nil || !strings.Contains(err.Error(), "failed") {
			t.Fatalf("want failed error, got %v", err)
		}
	})

	t.Run("failed process with message", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("POST", targetPost,
			httpmock.NewStringResponder(200, `{"process_id":"xyz"}`))

		httpmock.RegisterResponder("GET", targetGet, httpmock.NewStringResponder(200, `{
		"process": {
			"process_id":"xyz",
			"status":"failed",
			"message":"No keys for export with current export settings"
		}
	}`))

		cli, err := client.NewClient(token, projectID, nil)
		if err != nil {
			t.Fatal(err)
		}
		d := download.NewDownloader(cli)

		buf := mustJSONBody(t, map[string]any{"format": "json"})

		_, err = d.FetchBundleAsync(context.Background(), buf)
		if err == nil {
			t.Fatalf("want error, got nil")
		}

		if !strings.Contains(err.Error(), "failed") {
			t.Fatalf("want failed error, got %v", err)
		}
		if !strings.Contains(err.Error(), "No keys for export") {
			t.Fatalf("want server message in error, got %v", err)
		}
	})

	t.Run("failed status with whitespace is normalized", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("POST", targetPost,
			httpmock.NewStringResponder(200, `{"process_id":"xyz"}`))

		// Note the trailing whitespace/newline in status.
		httpmock.RegisterResponder("GET", targetGet, httpmock.NewStringResponder(200, `{
		"process": {
			"process_id":"xyz",
			"status":"failed \n",
			"message":"boom"
		}
	}`))

		cli, err := client.NewClient(token, projectID, nil)
		if err != nil {
			t.Fatal(err)
		}
		d := download.NewDownloader(cli)

		buf := mustJSONBody(t, map[string]any{"format": "json"})

		_, err = d.FetchBundleAsync(context.Background(), buf)
		if err == nil {
			t.Fatalf("want error, got nil")
		}

		// This ensures we didn't fall into the default branch ("did not finish").
		if strings.Contains(err.Error(), "did not finish") {
			t.Fatalf("status should be normalized to failed, got %v", err)
		}
		if !strings.Contains(err.Error(), "failed: boom") {
			t.Fatalf("want failed with message, got %v", err)
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

		d := download.NewDownloader(cli)

		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		buf := mustJSONBody(t, map[string]any{"format": "json"})

		_, err = d.FetchBundleAsync(ctx, buf)
		if err == nil || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("want context deadline, got %v", err)
		}
	})
}

func TestDownloader_FetchBundleAsync_FinishedButEmptyDownloadURL(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	targetPost := fmt.Sprintf("https://api.lokalise.com/api2/projects/%s/files/async-download", projectID)
	targetGet := fmt.Sprintf("https://api.lokalise.com/api2/projects/%s/processes/xyz", projectID)

	// kickoff ok
	httpmock.RegisterResponder("POST", targetPost,
		httpmock.NewStringResponder(200, `{"process_id":"xyz"}`))

	// finished, but download_url missing/empty
	httpmock.RegisterResponder("GET", targetGet, httpmock.NewStringResponder(200, `{
		"process": {
			"process_id":"xyz",
			"status":"finished",
			"details": {}
		}
	}`))

	cli, _ := client.NewClient(token, projectID, nil)
	d := download.NewDownloader(cli)

	buf := mustJSONBody(t, map[string]any{"format": "json"})

	_, err := d.FetchBundleAsync(context.Background(), buf)
	if err == nil || !strings.Contains(err.Error(), "download_url is empty") {
		t.Fatalf("want empty download_url error, got %v", err)
	}
}

func TestDownloader_FetchBundleAsync_NilBody(t *testing.T) {
	cli, _ := client.NewClient(token, projectID, nil)
	d := download.NewDownloader(cli)

	_, err := d.FetchBundleAsync(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "nil request body") {
		t.Fatalf("want nil request body error, got %v", err)
	}
}

func TestDownloader_FetchBundleAsync_EmptyProcessID(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	targetPost := fmt.Sprintf("https://api.lokalise.com/api2/projects/%s/files/async-download", projectID)
	httpmock.RegisterResponder("POST", targetPost, httpmock.NewStringResponder(200, `{"process_id":""}`))

	cli, _ := client.NewClient(token, projectID, nil)
	d := download.NewDownloader(cli)

	buf := mustJSONBody(t, map[string]any{"format": "json"})
	_, err := d.FetchBundleAsync(context.Background(), buf)

	if err == nil || !strings.Contains(err.Error(), "empty process id") {
		t.Fatalf("want empty process id error, got %v", err)
	}
}

func TestFetchBundleAsyncPrecheck(t *testing.T) {
	t.Run("nil downloader", func(t *testing.T) {
		t.Parallel()

		var d *download.Downloader

		gotCtx, err := download.ExportFetchBundleAsyncPrecheck(
			d,
			context.Background(),
			strings.NewReader(`{}`),
		)
		if err == nil {
			t.Fatal("FetchBundleAsyncPrecheck() error = nil, want non-nil")
		}
		if err.Error() != "fetch bundle async: nil downloader/client" {
			t.Fatalf("error = %q, want %q", err.Error(), "fetch bundle async: nil downloader/client")
		}
		if gotCtx != nil {
			t.Fatal("context != nil on error, want nil")
		}
	})

	t.Run("nil client", func(t *testing.T) {
		t.Parallel()

		d := download.ExportNewDownloaderWithClientForTest(nil)

		gotCtx, err := download.ExportFetchBundleAsyncPrecheck(
			d,
			context.Background(),
			strings.NewReader(`{}`),
		)
		if err == nil {
			t.Fatal("FetchBundleAsyncPrecheck() error = nil, want non-nil")
		}
		if err.Error() != "fetch bundle async: nil downloader/client" {
			t.Fatalf("error = %q, want %q", err.Error(), "fetch bundle async: nil downloader/client")
		}
		if gotCtx != nil {
			t.Fatal("context != nil on error, want nil")
		}
	})

	t.Run("nil context uses background", func(t *testing.T) {
		t.Parallel()

		cli, err := client.NewClient(token, projectID, nil)
		if err != nil {
			t.Fatal(err)
		}

		d := download.NewDownloader(cli)

		gotCtx, err := download.ExportFetchBundleAsyncPrecheck(
			d,
			nil,
			strings.NewReader(`{}`),
		)
		if err != nil {
			t.Fatalf("FetchBundleAsyncPrecheck() unexpected error = %v", err)
		}
		if gotCtx == nil {
			t.Fatal("context = nil, want non-nil")
		}
		if gotCtx.Err() != nil {
			t.Fatalf("context error = %v, want nil", gotCtx.Err())
		}
	})

	t.Run("canceled context", func(t *testing.T) {
		t.Parallel()

		cli, err := client.NewClient(token, projectID, nil)
		if err != nil {
			t.Fatal(err)
		}

		d := download.NewDownloader(cli)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		gotCtx, err := download.ExportFetchBundleAsyncPrecheck(
			d,
			ctx,
			strings.NewReader(`{}`),
		)
		if err == nil {
			t.Fatal("FetchBundleAsyncPrecheck() error = nil, want non-nil")
		}
		if err.Error() != "fetch bundle async: context: context canceled" {
			t.Fatalf("error = %q, want %q", err.Error(), "fetch bundle async: context: context canceled")
		}
		if gotCtx != nil {
			t.Fatal("context != nil on error, want nil")
		}
	})
}

func TestDownloader_StartAsyncDownload(t *testing.T) {
	targetPost := "https://api.lokalise.com/api2/projects/" + projectID + "/files/async-download"

	t.Run("DoJSONWithRetry error is wrapped", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("POST", targetPost,
			httpmock.NewErrorResponder(errors.New("post boom")))

		cli, err := client.NewClient(token, projectID, nil)
		if err != nil {
			t.Fatal(err)
		}

		d := download.NewDownloader(cli)

		_, err = download.ExportStartAsyncDownload(
			d,
			context.Background(),
			mustJSONBody(t, map[string]any{"format": "json"}),
		)
		if err == nil {
			t.Fatal("StartAsyncDownload() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "fetch bundle async:") {
			t.Fatalf("error = %q, want wrapped fetch bundle async error", err.Error())
		}
	})

	t.Run("empty process id", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("POST", targetPost,
			httpmock.NewStringResponder(200, `{"process_id":"   "}`))

		cli, err := client.NewClient(token, projectID, nil)
		if err != nil {
			t.Fatal(err)
		}

		d := download.NewDownloader(cli)

		_, err = download.ExportStartAsyncDownload(
			d,
			context.Background(),
			mustJSONBody(t, map[string]any{"format": "json"}),
		)
		if err == nil {
			t.Fatal("StartAsyncDownload() error = nil, want non-nil")
		}
		if err.Error() != "fetch bundle async: empty process id" {
			t.Fatalf("error = %q, want %q", err.Error(), "fetch bundle async: empty process id")
		}
	})
}

func TestPollAsyncDownloadProcess(t *testing.T) {
	t.Run("no process results returned", func(t *testing.T) {
		restore := download.ExportSetPollProcessesForTest(
			func(context.Context, []string, *client.Client) ([]background.QueuedProcess, error) {
				return []background.QueuedProcess{}, nil
			},
		)
		defer restore()

		cli, err := client.NewClient(token, projectID, nil)
		if err != nil {
			t.Fatal(err)
		}

		d := download.NewDownloader(cli)

		_, err = download.ExportPollAsyncDownloadProcess(d, context.Background(), "xyz")
		if err == nil {
			t.Fatal("PollAsyncDownloadProcess() error = nil, want non-nil")
		}
		if err.Error() != "fetch bundle async: no process results returned (process_id=xyz)" {
			t.Fatalf(
				"error = %q, want %q",
				err.Error(),
				"fetch bundle async: no process results returned (process_id=xyz)",
			)
		}
	})

	t.Run("poll processes error is wrapped", func(t *testing.T) {
		restore := download.ExportSetPollProcessesForTest(
			func(context.Context, []string, *client.Client) ([]background.QueuedProcess, error) {
				return nil, errors.New("poll boom")
			},
		)
		defer restore()

		cli, err := client.NewClient(token, projectID, nil)
		if err != nil {
			t.Fatal(err)
		}

		d := download.NewDownloader(cli)

		_, err = download.ExportPollAsyncDownloadProcess(d, context.Background(), "xyz")
		if err == nil {
			t.Fatal("PollAsyncDownloadProcess() error = nil, want non-nil")
		}
		if err.Error() != "fetch bundle async: poll processes: poll boom" {
			t.Fatalf(
				"error = %q, want %q",
				err.Error(),
				"fetch bundle async: poll processes: poll boom",
			)
		}
	})
}

func TestInterpretAsyncDownloadProcess(t *testing.T) {
	t.Parallel()

	t.Run("default status returns did not finish", func(t *testing.T) {
		t.Parallel()

		_, err := download.ExportInterpretAsyncDownloadProcess(background.QueuedProcess{
			ProcessID: "xyz",
			Status:    "queued",
		})
		if err == nil {
			t.Fatal("InterpretAsyncDownloadProcess() error = nil, want non-nil")
		}
		if err.Error() != `fetch bundle async: process xyz did not finish (status="queued")` {
			t.Fatalf(
				"error = %q, want %q",
				err.Error(),
				`fetch bundle async: process xyz did not finish (status="queued")`,
			)
		}
	})

	t.Run("finished with empty download url and message", func(t *testing.T) {
		t.Parallel()

		_, err := download.ExportFinishedAsyncDownloadURL(background.QueuedProcess{
			ProcessID:   "xyz",
			DownloadURL: "   ",
			Message:     " no url available ",
		})
		if err == nil {
			t.Fatal("FinishedAsyncDownloadURL() error = nil, want non-nil")
		}
		if err.Error() != "fetch bundle async: process xyz finished but download_url is empty: no url available" {
			t.Fatalf(
				"error = %q, want %q",
				err.Error(),
				"fetch bundle async: process xyz finished but download_url is empty: no url available",
			)
		}
	})
}
