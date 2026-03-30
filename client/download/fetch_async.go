package download

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/bodrovis/lokex/v2/client/internal/background"
	"github.com/bodrovis/lokex/v2/internal/utils"
)

// AsyncDownloadResponse is the minimal response payload returned by
// POST /files/async-download.
type AsyncDownloadResponse struct {
	ProcessID string `json:"process_id"`
}

// FetchBundleAsync kicks off an async export (POST /files/async-download) and polls
// until the process yields a terminal status. On success it returns download_url.
func (d *Downloader) FetchBundleAsync(ctx context.Context, body io.Reader) (string, error) {
	if d == nil || d.client == nil {
		return "", fmt.Errorf("fetch bundle async: nil downloader/client")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("fetch bundle async: context: %w", err)
	}
	if body == nil {
		return "", fmt.Errorf("fetch bundle async: nil request body")
	}

	// 1) Kick off async export -> get process_id.
	var kickoff AsyncDownloadResponse
	path := utils.ProjectPath(d.client.ProjectID, "files/async-download")

	if err := d.client.DoJSONWithRetry(ctx, http.MethodPost, path, body, &kickoff); err != nil {
		return "", fmt.Errorf("fetch bundle async: %w", err)
	}

	pid := strings.TrimSpace(kickoff.ProcessID)
	if pid == "" {
		return "", fmt.Errorf("fetch bundle async: empty process id")
	}

	// 2) Poll this single process until terminal or ctx/poll budget expires.
	results, err := background.PollProcesses(ctx, []string{pid}, d.client)
	if err != nil {
		return "", fmt.Errorf("fetch bundle async: poll processes: %w", err)
	}
	if len(results) == 0 {
		return "", fmt.Errorf("fetch bundle async: no process results returned (process_id=%s)", pid)
	}

	p := results[0]
	st := utils.NormalizeString(p.Status)

	// 3) Interpret result.
	switch st {
	case background.StatusFinished:
		u := strings.TrimSpace(p.DownloadURL)
		if u == "" {
			if msg := strings.TrimSpace(p.Message); msg != "" {
				return "", fmt.Errorf("fetch bundle async: process %s finished but download_url is empty: %s", p.ProcessID, msg)
			}
			return "", fmt.Errorf("fetch bundle async: process %s finished but download_url is empty", p.ProcessID)
		}

		return u, nil

	case background.StatusFailed:
		msg := strings.TrimSpace(p.Message)
		if msg != "" {
			return "", fmt.Errorf("fetch bundle async: process %s failed: %s", p.ProcessID, msg)
		}
		return "", fmt.Errorf("fetch bundle async: process %s failed", p.ProcessID)

	default:
		// Usually means we ran out of polling budget (PollMaxWait) but ctx might still be alive,
		// or Lokalise is slow and never reached terminal before our poll deadline.
		return "", fmt.Errorf("fetch bundle async: process %s did not finish (status=%q)", p.ProcessID, st)
	}
}
