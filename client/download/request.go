package download

import (
	"context"
	"fmt"
	"net/http"
)

// doDownloadRequest builds and executes a GET request for downloading raw zip data.

func (d *Downloader) doDownloadRequest(
	ctx context.Context,
	httpc *http.Client,
	urlStr, ua string,
) (*http.Response, error) {
	if httpc == nil {
		return nil, fmt.Errorf("download: nil HTTP client")
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		urlStr,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	if ua != "" {
		req.Header.Set("User-Agent", ua)
	}

	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("Accept", "application/zip, application/octet-stream, */*")

	safeClient := &http.Client{
		Transport: httpc.Transport,
		Timeout:   httpc.Timeout,
		Jar:       httpc.Jar,
		CheckRedirect: func(next *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("download: too many redirects")
			}

			if _, err := validateBundleURL(next.URL.String()); err != nil {
				return fmt.Errorf("download: invalid redirect: %w", err)
			}

			if httpc.CheckRedirect != nil {
				return httpc.CheckRedirect(next, via)
			}

			return nil
		},
	}

	resp, err := safeClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download request: %w", err)
	}

	return resp, nil
}
