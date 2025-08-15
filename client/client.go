package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	defaultBaseURL   = "https://api.lokalise.com/api2/"
	defaultUserAgent = "lokex/0.1"
)

type Client struct {
	BaseURL   string
	Token     string
	ProjectID string
	UserAgent string
}

type ClientOptions struct {
	BaseURL string
}

type APIError struct {
	Status  int
	Code    int
	Message string
	Raw     string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return http.StatusText(e.Status)
}

func NewClient(token, projectID string, opts *ClientOptions) *Client {
	baseURL := defaultBaseURL
	if opts != nil && opts.BaseURL != "" {
		baseURL = opts.BaseURL
	}
	if baseURL[len(baseURL)-1] != '/' {
		baseURL += "/"
	}
	return &Client{
		BaseURL:   baseURL,
		Token:     token,
		ProjectID: projectID,
		UserAgent: defaultUserAgent,
	}
}

// do sends the request, returns non-2xx as *APIError, and optionally decodes JSON into v.
func (c *Client) do(ctx context.Context, method, path string, body io.Reader, v any) (*http.Response, error) {
	url := c.BaseURL + path

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("X-Api-Token", c.Token)
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	// Non-2xx → parse Lokalise error: {"error":{"message":"...", "code":400}}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		slurp, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))

		var payload struct {
			Error struct {
				Message string `json:"message"`
				Code    int    `json:"code"`
			} `json:"error"`
		}
		_ = json.Unmarshal(slurp, &payload)

		return resp, &APIError{
			Status:  resp.StatusCode,
			Code:    payload.Error.Code,
			Message: coalesce(payload.Error.Message, strings.TrimSpace(string(slurp)), http.StatusText(resp.StatusCode)),
			Raw:     string(slurp),
		}
	}

	// Success: decode JSON if requested
	if v != nil {
		defer resp.Body.Close()
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
			return resp, fmt.Errorf("decode response: %w", err)
		}
	}

	return resp, nil
}

func coalesce(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

// helper to build "projects/{id}/<suffix>"
func (c *Client) projectPath(suffix string) string {
	return fmt.Sprintf("projects/%s/%s", c.ProjectID, suffix)
}
