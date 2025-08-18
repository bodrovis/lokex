package client

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bodrovis/lokex/apierr"
)

type Downloader struct {
	client *Client
}
type DownloadBundle struct {
	BundleURL string `json:"bundle_url"`
}
type DownloadParams map[string]any

func NewDownloader(c *Client) *Downloader {
	return &Downloader{
		client: c,
	}
}

func (d *Downloader) Download(ctx context.Context, unzipTo, format string, params DownloadParams) (string, error) {
	bundle, err := d.FetchBundle(ctx, format, params)
	if err != nil {
		return "", err
	}

	err = d.DownloadAndUnzip(ctx, bundle, unzipTo)
	if err != nil {
		return "", err
	}

	return bundle, nil
}

func (d *Downloader) FetchBundle(ctx context.Context, format string, params DownloadParams) (string, error) {
	if strings.TrimSpace(format) == "" {
		return "", fmt.Errorf("fetch bundle: format is required")
	}

	body := make(map[string]any, 1+len(params))
	body["format"] = format
	for k, v := range params {
		// don't let callers override the required field
		if strings.EqualFold(k, "format") {
			continue
		}
		body[k] = v
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(body); err != nil {
		return "", fmt.Errorf("fetch bundle: encode body: %w", err)
	}

	var bundle DownloadBundle
	path := d.client.projectPath("files/download")

	_, err := d.client.doWithRetry(ctx, http.MethodPost, path, &buf, &bundle)
	if err != nil {
		return "", fmt.Errorf("fetch bundle: %w", err)
	}
	if bundle.BundleURL == "" {
		return "", fmt.Errorf("fetch bundle: empty bundle url")
	}
	return bundle.BundleURL, nil
}

func (d *Downloader) DownloadAndUnzip(ctx context.Context, bundleURL, destDir string) error {
	if strings.TrimSpace(bundleURL) == "" {
		return fmt.Errorf("download: empty bundle url")
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("download: create dest: %w", err)
	}

	// temp ZIP (system temp dir)
	tmpZip, err := os.CreateTemp("", "lokex-*.zip")
	if err != nil {
		return fmt.Errorf("download: create temp zip: %w", err)
	}
	tmpPath := tmpZip.Name()
	_ = tmpZip.Close() // we'll reopen inside downloadOnce()
	defer func() { _ = os.Remove(tmpPath) }()

	ua := ""
	if d.client != nil {
		ua = d.client.UserAgent
	}

	err = d.client.withExpBackoff(ctx, "download", func(_ int) error {
		return d.downloadOnce(ctx, bundleURL, tmpPath, ua)
	}, nil)
	if err != nil {
		return err
	}

	// Unzip safely into destDir
	if err := unzipSafe(tmpPath, destDir); err != nil {
		return fmt.Errorf("unzip: %w", err)
	}
	return nil
}

func (d *Downloader) downloadOnce(ctx context.Context, url, destPath, ua string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if ua != "" {
		req.Header.Set("User-Agent", ua)
	}

	hc := d.client.HTTPClient
	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slurp, _ := io.ReadAll(io.LimitReader(resp.Body, defaultErrCap))
		return &apierr.APIError{
			Status:  resp.StatusCode,
			Message: strings.TrimSpace(string(slurp)),
			Code:    resp.StatusCode,
		}
	}

	f, err := os.Create(destPath) // truncate/overwrite
	if err != nil {
		return fmt.Errorf("create temp zip: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("write zip: %w", err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("flush zip: %w", err)
	}
	return nil
}

func unzipSafe(srcZip, destDir string) error {
	r, err := zip.OpenReader(srcZip)
	if err != nil {
		return err
	}
	defer r.Close()

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	destAbs, err := filepath.Abs(destDir)
	if err != nil {
		return err
	}

	for _, f := range r.File {
		// Normalize path inside the zip (remove leading slashes, clean ..)
		rel := filepath.Clean(f.Name)
		rel = strings.TrimPrefix(rel, "/")
		rel = strings.TrimPrefix(rel, string(filepath.Separator))
		targetPath := filepath.Join(destDir, rel)

		// zip-slip guard: ensure final path is still under destDir
		targetAbs, err := filepath.Abs(targetPath)
		if err != nil {
			return err
		}
		if targetAbs != destAbs && !strings.HasPrefix(targetAbs, destAbs+string(filepath.Separator)) {
			return fmt.Errorf("unsafe path in zip: %q", f.Name)
		}

		mode := f.Mode()

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(targetAbs, mode.Perm()); err != nil {
				return err
			}
			continue
		}

		// (Optional) Skip symlinks for portability/safety. If you do want them,
		// open f and read the link target; create os.Symlink safely.
		if mode&os.ModeSymlink != 0 {
			// skip silently; or: return fmt.Errorf("symlinks not supported: %s", f.Name)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetAbs), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(targetAbs, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode.Perm())
		if err != nil {
			rc.Close()
			return err
		}
		_, copyErr := io.Copy(out, rc)
		rc.Close()
		if cerr := out.Close(); copyErr == nil && cerr != nil {
			copyErr = cerr
		}
		if copyErr != nil {
			return copyErr
		}
	}
	return nil
}
