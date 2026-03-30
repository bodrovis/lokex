// Package client provides shared Lokalise client state and convenience helpers
// used by higher-level upload, download, and background workflows.
package client

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bodrovis/lokex/v2/client/internal/retry"
	"github.com/bodrovis/lokex/v2/client/internal/transport"
)

// It is intended to be safe for concurrent use after construction, assuming
// its fields are not mutated after NewClient returns. The embedded http.Client
// is used as-is.
type Client struct {
	BaseURL         string        // normalized base URL with trailing slash
	Token           string        // API token (X-Api-Token header)
	ProjectID       string        // default project ID for project-scoped endpoints
	UserAgent       string        // User-Agent header value
	HTTPClient      *http.Client  // underlying HTTP client
	MaxRetries      int           // number of retries after first attempt
	InitialBackoff  time.Duration // initial backoff duration for retries
	MaxBackoff      time.Duration // cap for backoff (and jittered sleep)
	PollInitialWait time.Duration // initial wait between PollProcesses rounds
	PollMaxWait     time.Duration // overall cap for PollProcesses duration
}

// NewClient builds a Client with sensible defaults and applies the provided
// options in order.
func NewClient(token, projectID string, opts ...Option) (*Client, error) {
	token = strings.TrimSpace(token)
	projectID = strings.TrimSpace(projectID)
	if token == "" {
		return nil, errors.New("API token is required")
	}
	if projectID == "" {
		return nil, errors.New("project ID is required")
	}

	c := &Client{
		BaseURL:         defaultBaseURL,
		Token:           token,
		ProjectID:       projectID,
		UserAgent:       defaultUserAgent,
		HTTPClient:      &http.Client{Timeout: defaultHTTPTimeout},
		MaxRetries:      defaultMaxRetries,
		InitialBackoff:  defaultInitialBackoff,
		MaxBackoff:      defaultMaxBackoff,
		PollInitialWait: defaultPollInitialWait,
		PollMaxWait:     defaultPollMaxWait,
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(c); err != nil {
			return nil, err
		}
	}

	return c, nil
}

// Requester builds a transport requester using the client's current HTTP settings.
func (c *Client) Requester() transport.Requester {
	return transport.Requester{
		BaseURL:    c.BaseURL,
		Token:      c.Token,
		UserAgent:  c.UserAgent,
		HTTPClient: c.HTTPClient,
	}
}

// DoJSONWithRetry performs one JSON request using the client's retry policy.
// If body supports replay (for example via retryBodyFactory or io.ReadSeeker),
// it may be retried without rebuilding the caller's request manually.
func (c *Client) DoJSONWithRetry(
	ctx context.Context,
	method, path string,
	body io.Reader,
	v any,
) error {
	reqr := c.Requester()
	return retry.DoWithRetry(
		ctx,
		retry.Config{
			Label:          "request",
			MaxRetries:     c.MaxRetries,
			InitialBackoff: c.InitialBackoff,
			MaxBackoff:     c.MaxBackoff,
		},
		body,
		func(_ int, b io.Reader) error {
			return reqr.DoJSON(ctx, method, path, b, v)
		},
		nil,
	)
}

// WithExpBackoff runs op using the client's retry/backoff settings.
func (c *Client) WithExpBackoff(
	ctx context.Context,
	label string,
	op func(attempt int) error,
	isRetryable func(error) bool,
) error {
	return retry.WithExpBackoff(
		ctx,
		label,
		c.MaxRetries,
		c.InitialBackoff,
		c.MaxBackoff,
		op,
		isRetryable,
	)
}
