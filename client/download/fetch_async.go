package download

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/bodrovis/lokex/v2/client"
	"github.com/bodrovis/lokex/v2/client/internal/background"
	"github.com/bodrovis/lokex/v2/internal/utils"
)

// AsyncDownloadResponse is the minimal response payload returned by
// POST /files/async-download.
type AsyncDownloadResponse struct {
	ProcessID string `json:"process_id"`
}

var pollProcessesFn = func(
	ctx context.Context,
	processIDs []string,
	c *client.Client,
) ([]background.QueuedProcess, error) {
	return background.PollProcesses(ctx, processIDs, c)
}

func ExportSetPollProcessesForTest(
	fn func(context.Context, []string, *client.Client) ([]background.QueuedProcess, error),
) func() {
	prev := pollProcessesFn
	pollProcessesFn = fn
	return func() {
		pollProcessesFn = prev
	}
}

// FetchBundleAsync kicks off an async export (POST /files/async-download) and polls
// until the process yields a terminal status. On success it returns download_url.
func (d *Downloader) FetchBundleAsync(ctx context.Context, body io.Reader) (string, error) {
	ctx, err := d.fetchBundleAsyncPrecheck(ctx, body)
	if err != nil {
		return "", err
	}

	pid, err := d.startAsyncDownload(ctx, body)
	if err != nil {
		return "", err
	}

	p, err := d.pollAsyncDownloadProcess(ctx, pid)
	if err != nil {
		return "", err
	}

	return interpretAsyncDownloadProcess(p)
}

func (d *Downloader) fetchBundleAsyncPrecheck(ctx context.Context, body io.Reader) (context.Context, error) {
	if d == nil || d.client == nil {
		return nil, fmt.Errorf("fetch bundle async: nil downloader/client")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("fetch bundle async: context: %w", err)
	}
	if body == nil {
		return nil, fmt.Errorf("fetch bundle async: nil request body")
	}
	return ctx, nil
}

func (d *Downloader) startAsyncDownload(ctx context.Context, body io.Reader) (string, error) {
	var kickoff AsyncDownloadResponse
	path := utils.ProjectPath(d.client.ProjectID, "files/async-download")

	if err := d.client.DoJSONWithRetry(ctx, http.MethodPost, path, body, &kickoff); err != nil {
		return "", fmt.Errorf("fetch bundle async: %w", err)
	}

	pid := strings.TrimSpace(kickoff.ProcessID)
	if pid == "" {
		return "", fmt.Errorf("fetch bundle async: empty process id")
	}

	return pid, nil
}

func (d *Downloader) pollAsyncDownloadProcess(ctx context.Context, pid string) (background.QueuedProcess, error) {
	results, err := pollProcessesFn(ctx, []string{pid}, d.client)
	if err != nil {
		return background.QueuedProcess{}, fmt.Errorf("fetch bundle async: poll processes: %w", err)
	}
	if len(results) == 0 {
		return background.QueuedProcess{}, fmt.Errorf(
			"fetch bundle async: no process results returned (process_id=%s)",
			pid,
		)
	}

	return results[0], nil
}

func interpretAsyncDownloadProcess(p background.QueuedProcess) (string, error) {
	st := utils.NormalizeString(p.Status)

	switch st {
	case background.StatusFinished:
		return finishedAsyncDownloadURL(p)

	case background.StatusFailed:
		return "", failedAsyncDownloadErr(p)

	default:
		// Usually means we ran out of polling budget (PollMaxWait) but ctx might still be alive,
		// or Lokalise is slow and never reached terminal before our poll deadline.
		return "", fmt.Errorf(
			"fetch bundle async: process %s did not finish (status=%q)",
			p.ProcessID,
			st,
		)
	}
}

func finishedAsyncDownloadURL(p background.QueuedProcess) (string, error) {
	u := strings.TrimSpace(p.DownloadURL)
	if u != "" {
		return u, nil
	}

	msg := strings.TrimSpace(p.Message)
	if msg != "" {
		return "", fmt.Errorf(
			"fetch bundle async: process %s finished but download_url is empty: %s",
			p.ProcessID,
			msg,
		)
	}

	return "", fmt.Errorf(
		"fetch bundle async: process %s finished but download_url is empty",
		p.ProcessID,
	)
}

func failedAsyncDownloadErr(p background.QueuedProcess) error {
	msg := strings.TrimSpace(p.Message)
	if msg != "" {
		return fmt.Errorf("fetch bundle async: process %s failed: %s", p.ProcessID, msg)
	}

	return fmt.Errorf("fetch bundle async: process %s failed", p.ProcessID)
}
