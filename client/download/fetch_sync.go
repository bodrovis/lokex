package download

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/bodrovis/lokex/v2/internal/utils"
)

// DownloadBundle is the minimal response payload returned by
// POST /files/download.
type DownloadBundle struct {
	BundleURL string `json:"bundle_url"`
}

// FetchBundle performs a synchronous export (POST /files/download) and returns the bundle URL.
func (d *Downloader) FetchBundle(ctx context.Context, body io.Reader) (string, error) {
	if d == nil || d.client == nil {
		return "", fmt.Errorf("fetch bundle: nil downloader/client")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("fetch bundle: context: %w", err)
	}
	if body == nil {
		return "", fmt.Errorf("fetch bundle: nil request body")
	}

	var bundle DownloadBundle
	path := utils.ProjectPath(d.client.ProjectID, "files/download")

	if err := d.client.DoJSONWithRetry(ctx, http.MethodPost, path, body, &bundle); err != nil {
		return "", fmt.Errorf("fetch bundle: %w", err)
	}

	bundleURL := strings.TrimSpace(bundle.BundleURL)
	if bundleURL == "" {
		return "", fmt.Errorf("fetch bundle: empty bundle url")
	}
	return bundleURL, nil
}
