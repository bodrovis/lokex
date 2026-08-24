package client_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/bodrovis/lokex/v2/client"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestNewClient_Defaults(t *testing.T) {
	t.Parallel()

	token := "tok123"
	projectID := "proj456"

	c, err := client.NewClient(token, projectID)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if c.Token != token {
		t.Fatalf("Token = %q, want %q", c.Token, token)
	}
	if c.ProjectID != projectID {
		t.Fatalf("ProjectID = %q, want %q", c.ProjectID, projectID)
	}
	if c.BaseURL != "https://api.lokalise.com/api2/" {
		t.Fatalf("BaseURL = %q, want %q", c.BaseURL, "https://api.lokalise.com/api2/")
	}
	if c.UserAgent != "lokex/2.4.0" {
		t.Fatalf("UserAgent = %q, want %q", c.UserAgent, "lokex/2.4.0")
	}
	if c.HTTPClient == nil {
		t.Fatal("HTTPClient = nil, want non-nil")
	}
	if c.HTTPClient.Timeout != 30*time.Second {
		t.Fatalf("HTTPClient.Timeout = %v, want %v", c.HTTPClient.Timeout, 30*time.Second)
	}
	if c.MaxRetries != 3 {
		t.Fatalf("MaxRetries = %d, want %d", c.MaxRetries, 3)
	}
	if c.InitialBackoff != 400*time.Millisecond {
		t.Fatalf("InitialBackoff = %v, want %v", c.InitialBackoff, 400*time.Millisecond)
	}
	if c.MaxBackoff != 5*time.Second {
		t.Fatalf("MaxBackoff = %v, want %v", c.MaxBackoff, 5*time.Second)
	}
	if c.PollInitialWait != 1*time.Second {
		t.Fatalf("PollInitialWait = %v, want %v", c.PollInitialWait, 1*time.Second)
	}
	if c.PollMaxWait != 120*time.Second {
		t.Fatalf("PollMaxWait = %v, want %v", c.PollMaxWait, 120*time.Second)
	}
}

func TestNewClient_RequiredFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		token     string
		projectID string
		wantErr   string
	}{
		{
			name:      "empty token after trim",
			token:     "   ",
			projectID: "proj456",
			wantErr:   "API token is required",
		},
		{
			name:      "empty projectID after trim",
			token:     "tok123",
			projectID: "   ",
			wantErr:   "project ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, err := client.NewClient(tt.token, tt.projectID)
			if err == nil {
				t.Fatal("NewClient() error = nil, want error")
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("error = %q, want %q", err.Error(), tt.wantErr)
			}
			if c != nil {
				t.Fatalf("client = %#v, want nil", c)
			}
		})
	}
}

func TestNewClient_TrimTokenAndProjectID(t *testing.T) {
	t.Parallel()

	c, err := client.NewClient("  tok123  ", "  proj456  ")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if c.Token != "tok123" {
		t.Fatalf("Token = %q, want %q", c.Token, "tok123")
	}
	if c.ProjectID != "proj456" {
		t.Fatalf("ProjectID = %q, want %q", c.ProjectID, "proj456")
	}
}

func TestDoJSONWithRetry_DoesNotKeepPartialDecodeStateBetweenAttempts(t *testing.T) {
	t.Parallel()

	const (
		firstBody  = `{"stale":"from-first","value":`
		secondBody = `{"value":"from-second"}`
	)

	attempts := 0

	hc := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			attempts++

			body := secondBody
			contentLength := int64(len(secondBody))

			if attempts == 1 {
				body = firstBody

				// Declare more bytes than are actually returned so the first
				// attempt is classified as a retryable transport truncation.
				contentLength = int64(len(firstBody) + 10)
			}

			return &http.Response{
				StatusCode:    http.StatusOK,
				Body:          io.NopCloser(strings.NewReader(body)),
				ContentLength: contentLength,
				Header:        make(http.Header),
				Request:       req,
			}, nil
		}),
	}

	c, err := client.NewClient(
		"tok",
		"project",
		client.WithHTTPClient(hc),
		client.WithMaxRetries(1),
		client.WithBackoff(time.Nanosecond, time.Nanosecond),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	var got struct {
		Stale string `json:"stale"`
		Value string `json:"value"`
	}

	err = c.DoJSONWithRetry(
		context.Background(),
		http.MethodGet,
		"/test",
		nil,
		&got,
	)
	if err != nil {
		t.Fatalf("DoJSONWithRetry() error = %v", err)
	}

	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}

	if got.Stale != "" {
		t.Fatalf(
			"Stale = %q, want empty; partial state from failed attempt was retained",
			got.Stale,
		)
	}

	if got.Value != "from-second" {
		t.Fatalf("Value = %q, want %q", got.Value, "from-second")
	}
}

func TestWithExpBackoff_UsesClientRetrySettings(t *testing.T) {
	t.Parallel()

	c, err := client.NewClient(
		"tok",
		"project",
		client.WithMaxRetries(2),
		client.WithBackoff(time.Nanosecond, time.Nanosecond),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	wantErr := errors.New("retry me")
	attempts := 0

	err = c.WithExpBackoff(
		context.Background(),
		"test operation",
		func(attempt int) error {
			attempts++

			if attempt < 2 {
				return wantErr
			}

			return nil
		},
		func(err error) bool {
			return errors.Is(err, wantErr)
		},
	)

	if err != nil {
		t.Fatalf("WithExpBackoff() error = %v", err)
	}

	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
}

func TestWithExpBackoff_StopsOnNonRetryableError(t *testing.T) {
	t.Parallel()

	c, err := client.NewClient(
		"tok",
		"project",
		client.WithMaxRetries(3),
		client.WithBackoff(time.Nanosecond, time.Nanosecond),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	wantErr := errors.New("do not retry")
	attempts := 0

	err = c.WithExpBackoff(
		context.Background(),
		"test operation",
		func(attempt int) error {
			attempts++
			return wantErr
		},
		func(error) bool {
			return false
		},
	)

	if !errors.Is(err, wantErr) {
		t.Fatalf("WithExpBackoff() error = %v, want %v", err, wantErr)
	}

	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}

func TestNewClient_NilOption(t *testing.T) {
	t.Parallel()

	c, err := client.NewClient(
		"tok",
		"project",
		nil,
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if c == nil {
		t.Fatal("NewClient() client = nil, want non-nil")
	}
}

func TestDoJSONWithRetry_NilDestination(t *testing.T) {
	t.Parallel()

	attempts := 0

	hc := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			attempts++

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`not valid json`)),
				Header:     make(http.Header),
				Request:    req,
			}, nil
		}),
	}

	c, err := client.NewClient(
		"tok",
		"project",
		client.WithHTTPClient(hc),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	err = c.DoJSONWithRetry(
		context.Background(),
		http.MethodGet,
		"/test",
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("DoJSONWithRetry() error = %v", err)
	}

	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}

func TestDoJSONWithRetry_InvalidDestination(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		v    any
	}{
		{
			name: "non-pointer",
			v:    struct{}{},
		},
		{
			name: "nil pointer",
			v:    (*struct{})(nil),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			hc := &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader(`{}`)),
						Header:     make(http.Header),
						Request:    req,
					}, nil
				}),
			}

			c, err := client.NewClient(
				"tok",
				"project",
				client.WithHTTPClient(hc),
				client.WithMaxRetries(0),
			)
			if err != nil {
				t.Fatalf("NewClient() error = %v", err)
			}

			err = c.DoJSONWithRetry(
				context.Background(),
				http.MethodGet,
				"/test",
				nil,
				tt.v,
			)
			if err == nil {
				t.Fatal("DoJSONWithRetry() error = nil, want non-nil")
			}
		})
	}
}
