package download

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bodrovis/lokex/v2/internal/zipx"
)

// DownloadAndUnzip downloads the zip from bundleURL with retry/backoff,
// validates that it's a well-formed zip, and unzips it into destDir with a
// series of safety checks (zip-slip, entry count, size caps, no symlinks/devs).
func (d *Downloader) DownloadAndUnzip(ctx context.Context, bundleURL, destDir string) error {
	if d == nil || d.client == nil || d.client.HTTPClient == nil {
		return fmt.Errorf("download: downloader/client/http client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("download: context: %w", err)
	}

	bundleURL = strings.TrimSpace(bundleURL)
	if bundleURL == "" {
		return fmt.Errorf("download: empty bundle url")
	}
	bundleURL, err := validateBundleURL(bundleURL)
	if err != nil {
		return err
	}
	destDir = strings.TrimSpace(destDir)
	if destDir == "" {
		return fmt.Errorf("download: empty dest dir")
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("download: create dest: %w", err)
	}

	// Temp dir per download attempt group. This keeps partial archives isolated
	// and makes cleanup straightforward.
	tmpDir, err := os.MkdirTemp("", "lokex-zip-*")
	if err != nil {
		return fmt.Errorf("download: create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	tmpPath := filepath.Join(tmpDir, "bundle.zip")

	ua := d.client.UserAgent

	// Retry the HTTP fetch + quick zip validation until success or policy expires.
	if err := d.client.WithExpBackoff(ctx, "download", func(_ int) error {
		if err := d.downloadOnce(ctx, bundleURL, tmpPath, ua); err != nil {
			return err
		}
		if err := zipx.Validate(tmpPath); err != nil {
			return fmt.Errorf("validate zip: %w", err)
		}
		return nil
	}, nil); err != nil {
		return err
	}

	if err := zipx.Unzip(tmpPath, destDir, zipx.DefaultPolicy()); err != nil {
		return fmt.Errorf("unzip: %w", err)
	}
	return nil
}
