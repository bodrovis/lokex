package download

import (
	"context"
	"fmt"
	"net/http"
)

// doDownloadRequest builds and executes a GET request for downloading raw zip data.
func (d *Downloader) doDownloadRequest(ctx context.Context, httpc *http.Client, urlStr, ua string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if ua != "" {
		req.Header.Set("User-Agent", ua)
	}
	// Avoid transparent compression; we want raw zip bytes on disk.
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("Accept", "application/zip, application/octet-stream, */*")

	resp, err := httpc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download request: %w", err)
	}
	return resp, nil
}
