package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bodrovis/lokex/apierr"
)

const (
	defaultBaseURL        = "https://api.lokalise.com/api2/"
	defaultUserAgent      = "lokex/0.1"
	defaultErrCap         = 8192
	defaultMaxRetries     = 3
	defaultInitialBackoff = 400 * time.Millisecond
	defaultMaxBackoff     = 5 * time.Second
)

type Client struct {
	BaseURL            string
	Token              string
	ProjectID          string
	UserAgent          string
	HTTPClient         *http.Client
	MaxDownloadRetries int
	InitialBackoff     time.Duration
	MaxBackoff         time.Duration
}

type ClientOptions struct {
	BaseURL            string
	HTTPClient         *http.Client
	MaxDownloadRetries int
	InitialBackoff     time.Duration
	MaxBackoff         time.Duration
}

func NewClient(token, projectID string, opts *ClientOptions) *Client {
	baseURL := defaultBaseURL
	if opts != nil && opts.BaseURL != "" {
		baseURL = opts.BaseURL
	}
	if baseURL[len(baseURL)-1] != '/' {
		baseURL += "/"
	}

	c := &Client{
		BaseURL:    baseURL,
		Token:      token,
		ProjectID:  projectID,
		UserAgent:  defaultUserAgent,
		HTTPClient: http.DefaultClient,
	}

	if opts != nil && opts.HTTPClient != nil {
		c.HTTPClient = opts.HTTPClient
	}

	// apply retry options if provided
	if opts != nil {
		c.MaxDownloadRetries = opts.MaxDownloadRetries
		c.InitialBackoff = opts.InitialBackoff
		c.MaxBackoff = opts.MaxBackoff
	}
	c.normalizeRetryDefaults()
	return c
}

// do sends the request, returns non-2xx as *APIError, and optionally decodes JSON into v.
func (c *Client) doWithRetry(ctx context.Context, method, path string, body io.Reader, v any) (*http.Response, error) {
	url := c.BaseURL + path

	// If there's a body, buffer it so we can retry safely.
	var payload []byte
	if body != nil {
		b, err := io.ReadAll(body)
		if err != nil {
			return nil, fmt.Errorf("buffer request body: %w", err)
		}
		payload = b
	}

	var outResp *http.Response
	err := c.withExpBackoff(ctx, "request", func(_ int) error {
		var rdr io.Reader
		if payload != nil {
			rdr = bytes.NewReader(payload)
		}

		req, err := http.NewRequestWithContext(ctx, method, url, rdr)
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("X-Api-Token", c.Token)
		req.Header.Set("User-Agent", c.UserAgent)
		req.Header.Set("Accept", "application/json")
		if rdr != nil {
			req.Header.Set("Content-Type", "application/json")
			req.ContentLength = int64(len(payload))
		}

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return fmt.Errorf("send request: %w", err)
		}

		// Non-2xx -> build *apierr.APIError so IsRetryable can decide (429/5xx)
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			defer resp.Body.Close()
			slurp, _ := io.ReadAll(io.LimitReader(resp.Body, defaultErrCap))
			ae := apierr.Parse(slurp, resp.StatusCode)
			ae.Resp = resp
			return ae
		}

		// Success: decode JSON if asked; if not, hand the resp to caller (they close it).
		if v != nil {
			defer resp.Body.Close()
			if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
				return fmt.Errorf("decode response: %w", err)
			}
		}
		outResp = resp
		return nil
	}, nil)
	if err != nil {
		return nil, err
	}
	return outResp, nil
}

func (c *Client) withExpBackoff(
	ctx context.Context,
	label string,
	op func(attempt int) error,
	isRetryable func(error) bool,
) error {
	if isRetryable == nil {
		isRetryable = apierr.IsRetryable
	}

	var lastErr error
	backoff := c.InitialBackoff

	for attempt := 0; ; attempt++ {
		if err := op(attempt); err == nil {
			return nil
		} else {
			lastErr = err
		}

		if !isRetryable(lastErr) || attempt >= c.MaxDownloadRetries {
			if label != "" {
				return fmt.Errorf("%s: %w", label, lastErr)
			}
			return lastErr
		}

		// jittered sleep capped at MaxBackoff; honor ctx
		delay := apierr.JitteredBackoff(backoff)
		delay = min(delay, c.MaxBackoff)

		select {
		case <-time.After(delay):
		case <-ctx.Done():
			if label != "" {
				return fmt.Errorf("%s: context: %w", label, ctx.Err())
			}
			return ctx.Err()
		}

		// grow backoff, capped
		backoff *= 2
		if backoff > c.MaxBackoff {
			backoff = c.MaxBackoff
		}
	}
}

// helper to build "projects/{id}/<suffix>"
func (c *Client) projectPath(suffix string) string {
	return fmt.Sprintf("projects/%s/%s", c.ProjectID, suffix)
}

func (c *Client) normalizeRetryDefaults() {
	if c.MaxDownloadRetries <= 0 {
		c.MaxDownloadRetries = defaultMaxRetries
	}
	if c.InitialBackoff <= 0 {
		c.InitialBackoff = defaultInitialBackoff
	}
	if c.MaxBackoff <= 0 {
		c.MaxBackoff = defaultMaxBackoff
	}
}
