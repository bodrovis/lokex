package client

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// defaultBaseURL is the production Lokalise REST API v2 base.
	defaultBaseURL = "https://api.lokalise.com/api2/"

	// defaultUserAgent is sent on every request unless overridden via WithUserAgent.
	defaultUserAgent = "lokex/2.0.0"

	// defaultErrCap caps how many bytes we slurp from a non-2xx response when
	// constructing an apierr.APIError.
	defaultErrCap = 8192

	// defaults for retry/backoff and HTTP timeouts.
	defaultMaxRetries     = 3
	defaultInitialBackoff = 400 * time.Millisecond
	defaultMaxBackoff     = 5 * time.Second
	defaultHTTPTimeout    = 30 * time.Second

	// defaults for the polling helper.
	defaultPollInitialWait = 1 * time.Second
	defaultPollMaxWait     = 120 * time.Second
)

// Option customizes a Client during construction.
// Errors returned by an Option abort NewClient.
type Option func(*Client) error

// WithBaseURL sets a custom API base URL.
// The value must be an absolute URL; a trailing slash is enforced.
func WithBaseURL(u string) Option {
	return func(c *Client) error {
		u = strings.TrimSpace(u)
		if u == "" {
			return errors.New("base URL cannot be empty")
		}
		parsed, err := url.Parse(u)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return errors.New("invalid base URL")
		}
		// normalize: ensure trailing slash and keep path/joining sane
		if !strings.HasSuffix(parsed.Path, "/") {
			parsed.Path += "/"
		}
		c.BaseURL = parsed.String()
		return nil
	}
}

// WithUserAgent overrides the default User-Agent string.
// An empty value is ignored.
func WithUserAgent(ua string) Option {
	return func(c *Client) error {
		ua = strings.TrimSpace(ua)
		if ua != "" {
			c.UserAgent = ua
		}
		return nil
	}
}

// WithHTTPClient replaces the underlying http.Client.
// The client must be non-nil.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) error {
		if hc == nil {
			return errors.New("http client cannot be nil")
		}
		c.HTTPClient = hc
		return nil
	}
}

// WithHTTPTimeout sets HTTP client timeout. If no HTTP client exists yet,
// a default one is created first.
// A zero value disables the timeout.
func WithHTTPTimeout(d time.Duration) Option {
	return func(c *Client) error {
		if d < 0 {
			return errors.New("http timeout cannot be negative")
		}
		if c.HTTPClient == nil {
			c.HTTPClient = &http.Client{Timeout: defaultHTTPTimeout}
		}
		c.HTTPClient.Timeout = d
		return nil
	}
}

// WithMaxRetries sets how many *retries* to attempt after the initial try.
// Use 0 (or negative) to disable retries entirely.
func WithMaxRetries(n int) Option {
	return func(c *Client) error {
		if n < 0 {
			n = 0
		}
		c.MaxRetries = n
		return nil
	}
}

// WithBackoff sets the exponential backoff window for retries.
// Zero/negative inputs fall back to library defaults.
// If max < initial, max is promoted to initial.
func WithBackoff(initial, max time.Duration) Option {
	return func(c *Client) error {
		if initial <= 0 {
			initial = defaultInitialBackoff
		}
		if max <= 0 {
			max = defaultMaxBackoff
		}
		if max < initial {
			max = initial
		}
		c.InitialBackoff = initial
		c.MaxBackoff = max
		return nil
	}
}

// WithPollWait sets the initial wait and the overall max wait for PollProcesses.
// Zero/negative inputs fall back to library defaults. If max < initial,
// max is promoted to initial.
func WithPollWait(initial, max time.Duration) Option {
	return func(c *Client) error {
		if initial <= 0 {
			initial = defaultPollInitialWait
		}
		if max <= 0 {
			max = defaultPollMaxWait
		}
		if max < initial {
			max = initial
		}
		c.PollInitialWait = initial
		c.PollMaxWait = max
		return nil
	}
}
