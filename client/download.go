package client

import (
	"context"
	"net/http"
)

type Downloader struct {
	client *Client
}
type DownloadBundle struct {
	BundleURL string `json:"bundle_url"`
}

func NewDownloader(c *Client) *Downloader {
	return &Downloader{client: c}
}

func (d *Downloader) Download(ctx context.Context) (string, error) {
	return d.FetchBundle(ctx)
}

func (d *Downloader) FetchBundle(ctx context.Context) (string, error) {
	var bundle DownloadBundle
	path := d.client.projectPath("files/download")

	_, err := d.client.do(ctx, http.MethodPost, path, nil, &bundle)
	if err != nil {
		return "", err
	}

	return bundle.BundleURL, nil
}
