// Package client provides a wrapper around the Lokalise API that the
// upload/download packages depend on. It handles base URL normalization,
// authentication, retry with exponential backoff,
// and simple polling of asynchronous processes.
package client

import (
	"errors"
	"net/http"
	"strings"
	"time"
)

// Client is a minimal Lokalise API client.
// It is safe for concurrent use after construction (fields are not mutated
// post-NewClient). The embedded http.Client is used as-is.
type Client struct {
	BaseURL         string        // normalized base URL with trailing slash
	Token           string        // API token (X-Api-Token header)
	ProjectID       string        // default project ID for project-scoped endpoints
	UserAgent       string        // User-Agent header value
	HTTPClient      *http.Client  // underlying HTTP client
	MaxRetries      int           // number of retries after first attempt
	InitialBackoff  time.Duration // first backoff duration for withExpBackoff
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

	// final normalization (in case WithBaseURL was not used)
	c.BaseURL = strings.TrimSpace(c.BaseURL)
	if !strings.HasSuffix(c.BaseURL, "/") {
		c.BaseURL += "/"
	}

	return c, nil
}
