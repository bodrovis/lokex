package client

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bodrovis/lokex/apierr"
	"github.com/bodrovis/lokex/utils"
)

type Downloader struct {
	client *Client
}
type DownloadBundle struct {
	BundleURL string `json:"bundle_url"`
}
type AsyncDownloadResponse struct {
	ProcessID string `json:"process_id"`
}

type DownloadParams map[string]any

func NewDownloader(c *Client) *Downloader {
	return &Downloader{
		client: c,
	}
}

func (d *Downloader) Download(ctx context.Context, unzipTo string, params DownloadParams) (string, error) {
	// detect async flag
	async := false
	if v, ok := params["async"]; ok {
		if bv, ok := v.(bool); ok && bv {
			async = true
		}
	}

	body := make(map[string]any, len(params))
	maps.Copy(body, params)
	delete(body, "async")

	var err error

	buf, err := utils.EncodeJSONBody(body)
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}

	var bundleURL string

	if async {
		bundleURL, err = d.FetchBundleAsync(ctx, buf)
	} else {
		bundleURL, err = d.FetchBundle(ctx, buf)
	}
	if err != nil {
		return "", err
	}

	err = d.DownloadAndUnzip(ctx, bundleURL, unzipTo)
	if err != nil {
		return "", err
	}

	return bundleURL, nil
}

func (d *Downloader) FetchBundleAsync(ctx context.Context, body io.Reader) (string, error) {
	var resp AsyncDownloadResponse
	path := d.client.projectPath("files/async-download")

	err := d.client.doWithRetry(ctx, http.MethodPost, path, body, &resp)
	if err != nil {
		return "", fmt.Errorf("fetch bundle async: %w", err)
	}
	if resp.ProcessID == "" {
		return "", fmt.Errorf("fetch bundle async: empty process id")
	}

	// Poll this single process until it finishes or times out
	results, err := d.client.PollProcesses(ctx, []string{resp.ProcessID})
	if err != nil {
		return "", fmt.Errorf("fetch bundle async: poll processes: %w", err)
	}
	if len(results) == 0 {
		return "", fmt.Errorf("fetch bundle async: no process results returned")
	}

	completed := results[0]
	if completed.Status == "finished" && completed.DownloadURL != "" {
		return completed.DownloadURL, nil
	}

	return "", fmt.Errorf(
		"fetch bundle async: process %s did not finish (status=%s)",
		completed.ProcessID,
		completed.Status,
	)
}

func (d *Downloader) FetchBundle(ctx context.Context, body io.Reader) (string, error) {
	var bundle DownloadBundle
	path := d.client.projectPath("files/download")

	err := d.client.doWithRetry(ctx, http.MethodPost, path, body, &bundle)
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

	if err := d.client.withExpBackoff(ctx, "download", func(_ int) error {
		if err := d.downloadOnce(ctx, bundleURL, tmpPath, ua); err != nil {
			return err
		}
		// validate it's a real zip; if not, return ErrUnexpectedEOF to trigger retry
		zr, zerr := zip.OpenReader(tmpPath)
		if zerr != nil {
			return fmt.Errorf("zip validate: %w", io.ErrUnexpectedEOF)
		}
		_ = zr.Close()
		return nil
	}, nil); err != nil {
		return err
	}

	// unzip after a validated download
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
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("Accept", "application/zip, application/octet-stream, */*")

	hc := d.client.HTTPClient
	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("http get: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slurp, _ := io.ReadAll(io.LimitReader(resp.Body, defaultErrCap))
		return &apierr.APIError{
			Status:  resp.StatusCode,
			Message: strings.TrimSpace(string(slurp)),
			Code:    resp.StatusCode,
		}
	}

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create temp zip: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()

	var want int64 = -1
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		if n, perr := strconv.ParseInt(cl, 10, 64); perr == nil && n >= 0 {
			want = n
		}
	}

	n, err := io.Copy(f, resp.Body)
	if err != nil {
		return fmt.Errorf("write zip: %w", err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("flush zip: %w", err)
	}

	// trigger retry if server cut us short
	if want >= 0 && n != want {
		return fmt.Errorf("incomplete download: got %d of %d: %w", n, want, io.ErrUnexpectedEOF)
	}
	return nil
}

func unzipSafe(srcZip, destDir string) error {
	r, err := zip.OpenReader(srcZip)
	if err != nil {
		return err
	}
	defer func() {
		_ = r.Close()
	}()

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	destAbs, err := filepath.Abs(destDir)
	if err != nil {
		return err
	}

	const (
		maxFiles       = 20000
		maxTotalUnzip  = 2 << 30   // 2 GiB
		maxSingleUnzip = 512 << 20 // 512 MiB
	)

	var total int64
	if len(r.File) > maxFiles {
		return fmt.Errorf("zip too many files: %d", len(r.File))
	}

	for _, f := range r.File {
		if f.UncompressedSize64 > maxSingleUnzip {
			return fmt.Errorf("zip entry too big: %s (%d bytes)", f.Name, f.UncompressedSize64)
		}
		if total += int64(f.UncompressedSize64); total > maxTotalUnzip {
			return fmt.Errorf("zip too large uncompressed: %d", total)
		}
		// Normalize path inside the zip (remove leading slashes, clean ..)
		rel := path.Clean(f.Name)          // not filepath.Clean
		rel = strings.TrimPrefix(rel, "/") // strip leading slashes
		if rel == "." {
			continue
		} // ignore weird root entries
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

		if mode&os.ModeSymlink != 0 || mode&os.ModeDevice != 0 || mode&os.ModeNamedPipe != 0 || mode&os.ModeSocket != 0 {
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
			defer func() {
				_ = rc.Close()
			}()
			return err
		}
		_, copyErr := io.Copy(out, rc)
		defer func() {
			_ = rc.Close()
		}()
		if cerr := out.Close(); copyErr == nil && cerr != nil {
			copyErr = cerr
		}
		if copyErr != nil {
			return copyErr
		}
	}
	return nil
}
