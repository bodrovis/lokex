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
	"time"

	"github.com/bodrovis/lokex/apierr"
)

const (
	defaultBaseURL         = "https://api.lokalise.com/api2/"
	defaultUserAgent       = "lokex/0.1"
	defaultErrCap          = 8192
	defaultMaxRetries      = 3
	defaultInitialBackoff  = 400 * time.Millisecond
	defaultMaxBackoff      = 5 * time.Second
	defaultHTTPTimeout     = 30 * time.Second
	defaultPollInitialWait = 1 * time.Second
	defaultPollMaxWait     = 120 * time.Second
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
	PollInitialWait    time.Duration
	PollMaxWait        time.Duration
}

type ClientOptions struct {
	BaseURL            string
	HTTPClient         *http.Client
	HTTPTimeout        time.Duration
	MaxDownloadRetries int
	InitialBackoff     time.Duration
	MaxBackoff         time.Duration
	PollInitialWait    time.Duration
	PollMaxWait        time.Duration
}

type QueuedProcess struct {
	ProcessID   string `json:"process_id"`
	Status      string `json:"status"`
	DownloadURL string `json:"download_url,omitempty"`
}

type processResponse struct {
	Process struct {
		ProcessID string `json:"process_id"`
		Status    string `json:"status"`
		Message   string `json:"message"`
		Details   struct {
			DownloadURL string `json:"download_url"`
		} `json:"details"`
	} `json:"process"`
}

func (pr *processResponse) ToQueuedProcess() QueuedProcess {
	return QueuedProcess{
		ProcessID:   pr.Process.ProcessID,
		Status:      pr.Process.Status,
		DownloadURL: pr.Process.Details.DownloadURL,
	}
}

// Option applies a customization to Client.
type Option func(*Client) error

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
			parsed.Path = parsed.Path + "/"
		}
		c.BaseURL = parsed.String()
		return nil
	}
}

func WithUserAgent(ua string) Option {
	return func(c *Client) error {
		ua = strings.TrimSpace(ua)
		if ua != "" {
			c.UserAgent = ua
		}
		return nil
	}
}

func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) error {
		if hc == nil {
			return errors.New("http client cannot be nil")
		}
		c.HTTPClient = hc
		return nil
	}
}

func WithHTTPTimeout(d time.Duration) Option {
	return func(c *Client) error {
		if c.HTTPClient == nil {
			c.HTTPClient = &http.Client{}
		}
		c.HTTPClient.Timeout = d
		return nil
	}
}

func WithMaxDownloadRetries(n int) Option {
	return func(c *Client) error {
		// allow 0 or negative on purpose if caller wants to disable
		c.MaxDownloadRetries = n
		return nil
	}
}

func WithBackoff(initial, max time.Duration) Option {
	return func(c *Client) error {
		// allow zero durations if caller wants to kill backoff
		c.InitialBackoff = initial
		c.MaxBackoff = max
		return nil
	}
}

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

// NewClient builds a client with defaults, then applies options.
// Zero values from options are treated as *explicit* values, not “unset”.
func NewClient(token, projectID string, opts ...Option) (*Client, error) {
	token = strings.TrimSpace(token)
	projectID = strings.TrimSpace(projectID)
	if token == "" {
		return nil, errors.New("token is required")
	}
	if projectID == "" {
		return nil, errors.New("project ID is required")
	}

	c := &Client{
		BaseURL:            defaultBaseURL,
		Token:              token,
		ProjectID:          projectID,
		UserAgent:          defaultUserAgent,
		HTTPClient:         &http.Client{Timeout: defaultHTTPTimeout},
		MaxDownloadRetries: defaultMaxRetries,
		InitialBackoff:     defaultInitialBackoff,
		MaxBackoff:         defaultMaxBackoff,
		PollInitialWait:    defaultPollInitialWait,
		PollMaxWait:        defaultPollMaxWait,
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(c); err != nil {
			return nil, err
		}
	}

	// final normalization
	if !strings.HasSuffix(c.BaseURL, "/") {
		c.BaseURL += "/"
	}

	return c, nil
}

func (client *Client) PollProcesses(ctx context.Context, processIDs []string) ([]QueuedProcess, error) {
	start := time.Now()
	wait := client.PollInitialWait
	maxWait := client.PollMaxWait

	processMap := make(map[string]QueuedProcess, len(processIDs))
	pending := make(map[string]struct{}, len(processIDs))

	for _, id := range processIDs {
		processMap[id] = QueuedProcess{ProcessID: id, Status: "queued"}
		pending[id] = struct{}{}
	}

	for len(pending) > 0 && time.Since(start) < maxWait {
		for id := range pending {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}

			path := client.projectPath(fmt.Sprintf("processes/%s", id))
			var resp processResponse

			_, err := client.doRequest(ctx, http.MethodGet, path, nil, &resp)
			if err != nil {
				// just skip this round, will retry in next loop iteration
				continue
			}

			proc := resp.ToQueuedProcess()
			processMap[id] = proc

			if proc.Status == "finished" || proc.Status == "failed" {
				delete(pending, id)
			}
		}

		if len(pending) == 0 {
			break
		}

		// wait before next poll iteration
		select {
		case <-time.After(wait):
			elapsed := time.Since(start)
			wait = wait * 2
			if wait > maxWait-elapsed {
				wait = maxWait - elapsed
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// collect results
	results := make([]QueuedProcess, 0, len(processMap))
	for _, p := range processMap {
		results = append(results, p)
	}
	return results, nil
}

func (c *Client) doWithRetry(ctx context.Context, method, path string, body io.Reader, v any) (*http.Response, error) {
	// If body isn't rewindable, buffer it once here so retries are safe
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

		resp, err := c.doRequest(ctx, method, path, rdr, v)
		if err != nil {
			return err
		}
		outResp = resp
		return nil
	}, nil)
	if err != nil {
		return nil, err
	}
	return outResp, nil
}

// doRequest performs a single HTTP request, no retries.
func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader, v any) (*http.Response, error) {
	url := c.BaseURL + path

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("X-Api-Token", c.Token)
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		// body may be non-rewindable, caller should handle if retries are needed
		if seeker, ok := body.(io.Seeker); ok {
			// safe: length from current pos to end
			if size, err := seeker.Seek(0, io.SeekEnd); err == nil {
				_, _ = seeker.Seek(0, io.SeekStart)
				req.ContentLength = size
			}
		}
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slurp, _ := io.ReadAll(io.LimitReader(resp.Body, defaultErrCap))
		ae := apierr.Parse(slurp, resp.StatusCode)
		ae.Resp = resp
		return resp, ae
	}

	if v != nil {
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
			return resp, fmt.Errorf("decode response: %w", err)
		}
	}

	return resp, nil
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

		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
			// wait for retry
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			if label != "" {
				return fmt.Errorf("%s: context: %w", label, ctx.Err())
			}
			return ctx.Err()
		}
		timer.Stop() // safe call

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

func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
