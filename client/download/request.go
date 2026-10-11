package download

import (
	"context"
	"fmt"
	"net/http"
)

// doDownloadRequest builds and executes a GET request for raw ZIP data.
func (d *Downloader) doDownloadRequest(
	ctx context.Context,
	httpc *http.Client,
	urlStr, ua string,
) (*http.Response, error) {
	if httpc == nil {
		return nil, fmt.Errorf("download: nil HTTP client")
	}

	req, err := newDownloadRequest(ctx, urlStr, ua)
	if err != nil {
		return nil, err
	}

	resp, err := newDownloadHTTPClient(httpc).Do(req)
	if err != nil {
		return nil, fmt.Errorf("download request: %w", err)
	}

	return resp, nil
}

// newDownloadRequest prepares a GET request with the required headers.
func newDownloadRequest(
	ctx context.Context,
	urlStr, ua string,
) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	if ua != "" {
		req.Header.Set("User-Agent", ua)
	}

	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("Accept", "application/zip, application/octet-stream, */*")

	return req, nil
}

// newDownloadHTTPClient preserves the caller's HTTP settings
// while enforcing URL validation on redirects.
func newDownloadHTTPClient(httpc *http.Client) *http.Client {
	return &http.Client{
		Transport: httpc.Transport,
		Timeout:   httpc.Timeout,
		Jar:       httpc.Jar,
		CheckRedirect: func(next *http.Request, via []*http.Request) error {
			return checkDownloadRedirect(next, via, httpc.CheckRedirect)
		},
	}
}

// checkDownloadRedirect validates each redirect and then applies
// the caller's custom redirect policy, if provided.
func checkDownloadRedirect(
	next *http.Request,
	via []*http.Request,
	original func(*http.Request, []*http.Request) error,
) error {
	if len(via) >= 10 {
		return fmt.Errorf("download: too many redirects")
	}

	if _, err := validateBundleURL(next.URL.String()); err != nil {
		return fmt.Errorf("download: invalid redirect: %w", err)
	}

	if original != nil {
		return original(next, via)
	}

	return nil
}
