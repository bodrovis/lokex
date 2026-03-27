package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/bodrovis/lokex/v2/internal/apierr"
)

type countingReader struct {
	r io.Reader
	n int64
}

// doRequest performs a single HTTP request (no retries).
// Body is sent as-is. If v is nil, the body is drained and discarded;
// otherwise it is decoded as JSON.
func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader, v any, headers http.Header) error {
	// Close request body ONLY if we fail before http.Client.Do().
	closeBody := func() {
		if body == nil {
			return
		}
		if cl, ok := body.(io.Closer); ok {
			_ = cl.Close()
		}
	}

	fullURL, err := url.JoinPath(c.BaseURL, path)
	if err != nil {
		closeBody()
		return fmt.Errorf("join url: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		closeBody()
		return fmt.Errorf("create request: %w", err)
	}

	// Best-effort Content-Length for common reader types (helps traces; avoids chunked uploads).
	switch b := body.(type) {
	case *bytes.Reader:
		req.ContentLength = int64(b.Len())
	case *strings.Reader:
		req.ContentLength = int64(b.Len())
	}

	req.Header.Set("X-Api-Token", c.Token)
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json")

	for k, vv := range headers {
		if len(vv) == 0 {
			continue
		}
		req.Header.Del(k)
		req.Header[k] = append([]string(nil), vv...)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		// after Do() net/http already handled closing the request body.
		return fmt.Errorf("send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Non-2xx: parse as APIError with a bounded snippet for debugging.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slurp, _ := io.ReadAll(io.LimitReader(resp.Body, defaultErrCap))
		// Drain the rest to maximize chances of connection reuse.
		_, _ = io.Copy(io.Discard, resp.Body)

		ae := apierr.Parse(slurp, resp.StatusCode)
		// Keep headers/status accessible; body is already consumed (don't read it).
		ae.Resp = resp
		return ae
	}

	// No target to decode into → drain body and return.
	if v == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}

	// Stream JSON decode to avoid buffering potentially large bodies,
	// but keep good error semantics for retry layer.
	cr := &countingReader{r: resp.Body}
	dec := json.NewDecoder(cr)

	if err := dec.Decode(v); err != nil {
		// Empty body (204 or some 200s) is fine.
		if errors.Is(err, io.EOF) {
			return nil
		}

		// Decoder returns io.ErrUnexpectedEOF for truncated JSON.
		// Distinguish "transport truncation" vs "server sent broken JSON".
		if errors.Is(err, io.ErrUnexpectedEOF) {
			// If Content-Length is known and we read less than promised,
			// treat as a truncated response -> retryable for higher layer.
			if resp.ContentLength > 0 && cr.n < resp.ContentLength {
				return fmt.Errorf("read response: %w", io.ErrUnexpectedEOF)
			}
			// Otherwise behave like json.Unmarshal on incomplete JSON:
			// stable message + NOT retryable.
			return fmt.Errorf("decode response: unexpected end of JSON input")
		}

		// Other decode errors (SyntaxError, type errors, etc.) -> non-retryable by default.
		return fmt.Errorf("decode response: %w", err)
	}

	// Strict trailing junk detection (optional). Keep if you want.
	if err := dec.Decode(new(struct{})); err == nil {
		return fmt.Errorf("decode response: trailing data")
	} else if !errors.Is(err, io.EOF) {
		return fmt.Errorf("decode response: %w", err)
	}

	return nil
}

func (cr *countingReader) Read(p []byte) (int, error) {
	n, err := cr.r.Read(p)
	cr.n += int64(n)
	return n, err
}
