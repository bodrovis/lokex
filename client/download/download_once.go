package download

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/bodrovis/lokex/v2/internal/apierr"
)

// downloadOnce performs a single GET of the bundle and writes it to destPath.
// It writes into a temp file first and renames it on success, so partial downloads
// never leave broken zips at destPath.
func (d *Downloader) downloadOnce(ctx context.Context, urlStr, destPath, ua string) error {
	httpc, urlStr, destPath, err := d.downloadOncePrecheck(ctx, urlStr, destPath)
	if err != nil {
		return err
	}

	resp, err := d.doDownloadRequest(ctx, httpc, urlStr, ua)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	// Non-2xx: read a capped snippet for an APIError and bail.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slurp, _ := io.ReadAll(io.LimitReader(resp.Body, apierr.DefaultErrCap))
		_, _ = io.Copy(io.Discard, resp.Body)
		return apierr.Parse(slurp, resp.StatusCode)
	}

	return writeHTTPBodyAtomically(destPath, resp.Body, resp.ContentLength)
}

// downloadOncePrecheck validates inputs and extracts the http.Client.
// Keeping this separate makes downloadOnce small and avoids nil-panics.
func (d *Downloader) downloadOncePrecheck(ctx context.Context, urlStr, destPath string) (*http.Client, string, string, error) {
	if d == nil || d.client == nil || d.client.HTTPClient == nil {
		return nil, "", "", fmt.Errorf("download: downloader/client/http client is nil")
	}
	if ctx == nil {
		return nil, "", "", fmt.Errorf("download: nil context")
	}
	if cerr := ctx.Err(); cerr != nil {
		return nil, "", "", cerr
	}

	urlStr = strings.TrimSpace(urlStr)
	destPath = strings.TrimSpace(destPath)
	if urlStr == "" {
		return nil, "", "", fmt.Errorf("download: empty url")
	}
	if destPath == "" {
		return nil, "", "", fmt.Errorf("download: empty dest path")
	}

	return d.client.HTTPClient, urlStr, destPath, nil
}
