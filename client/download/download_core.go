// Package download provides a downloader for Lokalise export bundles.
//
// This file provides a small helper around the two download flows Lokalise
// supports:
//
//   - Synchronous download: POST /files/download → returns a bundle_url (zip).
//   - Asynchronous download: POST /files/async-download → returns process_id,
//     which is then polled via /processes/{id} until it yields a download_url.
//
// The downloader will fetch the bundle URL (sync or async), download the zip
// with retry/backoff, validate it's a real zip, and then safely unzip into the
// provided destination directory with zip-slip and size guards.
package download

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"strings"

	"github.com/bodrovis/lokex/v2/client"
	"github.com/bodrovis/lokex/v2/internal/utils"
)

// Downloader wraps a *Client to perform Lokalise file exports (downloads).
// Construct with NewDownloader; the embedded client must be non-nil.
type Downloader struct {
	client *client.Client
}

// DownloadParams represents the JSON body for /files/download and /files/async-download.
// It's a thin alias so callers can pass a map with a clearer domain-specific type.
type DownloadParams map[string]any

// FetchFunc abstracts the "fetch bundle URL" step so Download and DownloadAsync
// can share the same pipeline.
type FetchFunc func(ctx context.Context, body io.Reader) (string, error)

var downloadAndUnzipFn = func(d *Downloader, ctx context.Context, bundleURL, destDir string) error {
	return d.DownloadAndUnzip(ctx, bundleURL, destDir)
}

var encodeJSONBody = utils.EncodeJSONBody

const clientIsNilMsg = "download: downloader/client is nil"

// NewDownloader creates a new Downloader bound to c.
// c must be non-nil; it is used for HTTP, retry/backoff, and polling.
func NewDownloader(c *client.Client) *Downloader {
	if c == nil {
		panic("lokex/download: nil client passed to NewDownloader")
	}
	return &Downloader{
		client: c,
	}
}

// Download performs a synchronous export:
//
//  1. POST /files/download with params
//  2. Receive bundle_url
//  3. Download the zip (with retry/backoff), validate, unzip to unzipTo
//
// Returns the bundle_url on success.
func (d *Downloader) Download(ctx context.Context, unzipTo string, params DownloadParams) (string, error) {
	if d == nil || d.client == nil {
		return "", errors.New(clientIsNilMsg)
	}
	return d.doDownload(ctx, unzipTo, params, d.FetchBundle)
}

// DownloadAsync performs an asynchronous export:
//
//  1. POST /files/async-download with params to get process_id
//  2. Poll /processes/{id} until status is finished
//  3. Receive download_url from the finished process
//  4. Download the zip (with retry/backoff), validate, unzip to unzipTo
//
// Returns the final download_url on success.
func (d *Downloader) DownloadAsync(ctx context.Context, unzipTo string, params DownloadParams) (string, error) {
	if d == nil || d.client == nil {
		return "", errors.New(clientIsNilMsg)
	}
	return d.doDownload(ctx, unzipTo, params, d.FetchBundleAsync)
}

// doDownload is the shared pipeline for both sync and async flows.
// It builds the JSON body, calls fetch() to obtain the bundle URL, downloads
// and validates the zip, and unzips into unzipTo. The returned string is the
// bundle URL used (sync: bundle_url; async: download_url).
func (d *Downloader) doDownload(
	ctx context.Context,
	unzipTo string,
	params DownloadParams,
	fetch FetchFunc,
) (string, error) {
	if d == nil || d.client == nil {
		return "", errors.New(clientIsNilMsg)
	}
	if fetch == nil {
		return "", errors.New("download: fetch func is nil")
	}
	if strings.TrimSpace(unzipTo) == "" {
		return "", errors.New("download: empty unzip destination")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("download: context: %w", err)
	}

	rdr, err := prepareBodyReader(params)
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}

	bundleURL, err := fetch(ctx, rdr)
	if err != nil {
		return "", err
	}

	if err := downloadAndUnzipFn(d, ctx, bundleURL, unzipTo); err != nil {
		return "", err
	}

	return bundleURL, nil
}

func prepareBodyReader(params DownloadParams) (*bytes.Reader, error) {
	// copy to avoid mutating caller's map
	var body map[string]any
	if len(params) > 0 {
		body = make(map[string]any, len(params))
		maps.Copy(body, params)
	} else {
		body = map[string]any{}
	}

	rdr, err := encodeJSONBody(body)
	if err != nil {
		return nil, err
	}

	return rdr, nil
}
