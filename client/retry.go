package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
)

type retryBodyFactory interface {
	// NewBody must return a fresh body for each attempt.
	NewBody() (io.ReadCloser, error)
}

// doWithRetry executes one HTTP operation and retries according to the client's
// backoff policy. If the body is seekable or provides a retryBodyFactory it is
// reused across attempts; otherwise it is buffered into memory once.
func (c *Client) doWithRetry(ctx context.Context, method, path string, body io.Reader, v any) error {
	headers := make(http.Header)
	if body != nil {
		headers.Set("Content-Type", "application/json")
	}

	// 1) Preferred: retryBodyFactory (fresh body each attempt).
	if f, ok := body.(retryBodyFactory); ok {
		return c.withExpBackoff(ctx, "request", func(_ int) error {
			rc, err := f.NewBody()
			if err != nil {
				return fmt.Errorf("create request body: %w", err)
			}
			// pass rc as-is (it is io.ReadCloser) so Transport can close it properly
			return c.doRequest(ctx, method, path, rc, v, headers)
		}, nil)
	}

	// 2) Seekable: rewind per attempt.
	// If it's also a Closer (e.g. *os.File), we must prevent net/http from closing it per attempt,
	// otherwise retries break. So we hide Close() and close once after all attempts.
	if rs, ok := body.(io.ReadSeeker); ok {
		if cl, ok := body.(io.Closer); ok {
			defer func() { _ = cl.Close() }()

			return c.withExpBackoff(ctx, "request", func(_ int) error {
				if _, err := rs.Seek(0, io.SeekStart); err != nil {
					return fmt.Errorf("rewind body: %w", err)
				}
				// Hide Close(): wrapper does NOT implement io.ReadCloser.
				rdr := struct{ io.Reader }{rs}
				return c.doRequest(ctx, method, path, rdr, v, headers)
			}, nil)
		}

		// ReadSeeker but not Closer: safe to pass directly.
		return c.withExpBackoff(ctx, "request", func(_ int) error {
			if _, err := rs.Seek(0, io.SeekStart); err != nil {
				return fmt.Errorf("rewind body: %w", err)
			}
			return c.doRequest(ctx, method, path, rs, v, headers)
		}, nil)
	}

	// 3) Fallback: buffer once (may allocate).
	var payload []byte
	if body != nil {
		b, err := io.ReadAll(body)
		if err != nil {
			return fmt.Errorf("buffer request body: %w", err)
		}
		if cbody, ok := body.(io.Closer); ok {
			_ = cbody.Close()
		}
		payload = b
	}

	return c.withExpBackoff(ctx, "request", func(_ int) error {
		var rdr io.Reader
		if payload != nil {
			rdr = bytes.NewReader(payload)
		}
		return c.doRequest(ctx, method, path, rdr, v, headers)
	}, nil)
}
