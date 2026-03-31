package download

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bodrovis/lokex/v2/internal/zipx"
)

var (
	mkdirAll  = os.MkdirAll
	mkdirTemp = os.MkdirTemp
	removeAll = os.RemoveAll
)

// DownloadAndUnzip downloads the zip from bundleURL with retry/backoff,
// validates that it's a well-formed zip, and unzips it into destDir with a
// series of safety checks (zip-slip, entry count, size caps, no symlinks/devs).
func (d *Downloader) DownloadAndUnzip(ctx context.Context, bundleURL, destDir string) error {
	ctx, bundleURL, destDir, err := d.downloadAndUnzipPrecheck(ctx, bundleURL, destDir)
	if err != nil {
		return err
	}

	if err := ensureDestDir(destDir); err != nil {
		return err
	}

	tmpDir, cleanup, err := createDownloadTempDir()
	if err != nil {
		return err
	}
	defer cleanup()

	tmpPath := filepath.Join(tmpDir, "bundle.zip")

	if err := d.downloadAndValidateZip(ctx, bundleURL, tmpPath); err != nil {
		return err
	}

	return unzipDownloadedBundle(tmpPath, destDir)
}

func (d *Downloader) downloadAndUnzipPrecheck(
	ctx context.Context,
	bundleURL, destDir string,
) (context.Context, string, string, error) {
	if d == nil || d.client == nil || d.client.HTTPClient == nil {
		return nil, "", "", fmt.Errorf("download: downloader/client/http client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, "", "", fmt.Errorf("download: context: %w", err)
	}

	bundleURL = strings.TrimSpace(bundleURL)
	if bundleURL == "" {
		return nil, "", "", fmt.Errorf("download: empty bundle url")
	}

	validatedURL, err := validateBundleURL(bundleURL)
	if err != nil {
		return nil, "", "", err
	}

	destDir = strings.TrimSpace(destDir)
	if destDir == "" {
		return nil, "", "", fmt.Errorf("download: empty dest dir")
	}

	return ctx, validatedURL, destDir, nil
}

func ensureDestDir(destDir string) error {
	if err := mkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("download: create dest: %w", err)
	}
	return nil
}

func createDownloadTempDir() (string, func(), error) {
	tmpDir, err := mkdirTemp("", "lokex-zip-*")
	if err != nil {
		return "", nil, fmt.Errorf("download: create temp dir: %w", err)
	}

	cleanup := func() {
		_ = removeAll(tmpDir)
	}

	return tmpDir, cleanup, nil
}

func (d *Downloader) downloadAndValidateZip(
	ctx context.Context,
	bundleURL, tmpPath string,
) error {
	ua := d.client.UserAgent

	return d.client.WithExpBackoff(ctx, "download", func(_ int) error {
		if err := d.downloadOnce(ctx, bundleURL, tmpPath, ua); err != nil {
			return err
		}
		if err := zipx.Validate(tmpPath); err != nil {
			return fmt.Errorf("validate zip: %w", err)
		}
		return nil
	}, nil)
}

func unzipDownloadedBundle(tmpPath, destDir string) error {
	if err := zipx.Unzip(tmpPath, destDir, zipx.DefaultPolicy()); err != nil {
		return fmt.Errorf("unzip: %w", err)
	}
	return nil
}
