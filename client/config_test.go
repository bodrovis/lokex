package client_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bodrovis/lokex/v2/client"
)

func TestNewClient_CustomBaseURL(t *testing.T) {
	token := "tok123"
	projectID := "proj456"
	customBase := "https://custom.lokalise.test/api2/"

	c, err := client.NewClient(token, projectID, client.WithBaseURL(
		customBase,
	))
	if err != nil {
		t.Fatalf("Cannot create client")
	}

	if c.BaseURL != customBase {
		t.Fatalf("BaseURL = %q, want %q", c.BaseURL, customBase)
	}
}

func TestNewClient_WithUserAgentAndHTTPTimeout(t *testing.T) {
	ua := "lokex-test/1.0"
	c, err := client.NewClient("t", "p",
		client.WithUserAgent(ua),
		client.WithHTTPTimeout(1500*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	// not directly asserted here; we check headers below in PollProcesses
	if c.HTTPClient.Timeout != 1500*time.Millisecond {
		t.Fatalf("timeout = %v, want 1.5s", c.HTTPClient.Timeout)
	}
}

func TestNewClient_OptionValidation(t *testing.T) {
	if _, err := client.NewClient(
		"t",
		"p",
		client.WithBaseURL(":// nope"),
	); err == nil {
		t.Fatal("expected error for invalid base URL")
	}

	if _, err := client.NewClient(
		"t",
		"p",
		client.WithHTTPClient(nil),
	); err == nil {
		t.Fatal("expected error for nil http client")
	}

	srv := httptest.NewTestServer(t, http.NotFoundHandler())

	// Initializes the test server and populates srv.URL.
	_ = srv.Client()

	baseNoSlash := srv.URL

	c, err := client.NewClient(
		"t",
		"p",
		client.WithBaseURL(baseNoSlash),
	)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	if got := c.BaseURL[len(c.BaseURL)-1:]; got != "/" {
		t.Fatalf("expected trailing slash, got %q", c.BaseURL)
	}
}

func TestWithBaseURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{
			name:  "accepts https",
			input: "https://example.com/api2/",
			want:  "https://example.com/api2/",
		},
		{
			name:  "accepts http",
			input: "http://example.com/api2/",
			want:  "http://example.com/api2/",
		},
		{
			name:  "trims whitespace",
			input: "  https://example.com/api2/  ",
			want:  "https://example.com/api2/",
		},
		{
			name:  "adds trailing slash",
			input: "https://example.com/api2",
			want:  "https://example.com/api2/",
		},
		{
			name:    "rejects empty",
			input:   " \t\n ",
			wantErr: "base URL cannot be empty",
		},
		{
			name:    "rejects malformed URL",
			input:   ":// nope",
			wantErr: "invalid base URL",
		},
		{
			name:    "rejects unsupported scheme",
			input:   "ftp://example.com/api2/",
			wantErr: "invalid base URL",
		},
		{
			name:    "rejects missing host",
			input:   "https:///api2/",
			wantErr: "invalid base URL",
		},
		{
			name:    "rejects fragment",
			input:   "https://example.com/api2/#x",
			wantErr: "invalid base URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := &client.Client{}
			err := client.WithBaseURL(tt.input)(c)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("WithBaseURL() error = nil, want non-nil")
				}
				if err.Error() != tt.wantErr {
					t.Fatalf("error = %q, want %q", err.Error(), tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("WithBaseURL() error = %v", err)
			}
			if c.BaseURL != tt.want {
				t.Fatalf("BaseURL = %q, want %q", c.BaseURL, tt.want)
			}
		})
	}
}

func TestWithUserAgent(t *testing.T) {
	t.Parallel()

	t.Run("sets trimmed value", func(t *testing.T) {
		t.Parallel()

		c := &client.Client{UserAgent: "old"}

		if err := client.WithUserAgent("  lokex-test/1.0  ")(c); err != nil {
			t.Fatalf("WithUserAgent() error = %v", err)
		}

		if c.UserAgent != "lokex-test/1.0" {
			t.Fatalf("UserAgent = %q, want %q", c.UserAgent, "lokex-test/1.0")
		}
	})

	t.Run("empty value is ignored", func(t *testing.T) {
		t.Parallel()

		c := &client.Client{UserAgent: "existing"}

		if err := client.WithUserAgent(" \t ")(c); err != nil {
			t.Fatalf("WithUserAgent() error = %v", err)
		}

		if c.UserAgent != "existing" {
			t.Fatalf("UserAgent = %q, want %q", c.UserAgent, "existing")
		}
	})
}

func TestWithHTTPClient_SetsClient(t *testing.T) {
	t.Parallel()

	hc := &http.Client{}
	c := &client.Client{}

	if err := client.WithHTTPClient(hc)(c); err != nil {
		t.Fatalf("WithHTTPClient() error = %v", err)
	}

	if c.HTTPClient != hc {
		t.Fatal("HTTPClient pointer differs from supplied client")
	}
}

func TestWithHTTPTimeout_ZeroDisablesTimeout(t *testing.T) {
	t.Parallel()

	hc := &http.Client{Timeout: 5 * time.Second}
	c := &client.Client{HTTPClient: hc}

	if err := client.WithHTTPTimeout(0)(c); err != nil {
		t.Fatalf("WithHTTPTimeout() error = %v", err)
	}

	if c.HTTPClient != hc {
		t.Fatal("HTTPClient pointer was replaced")
	}
	if c.HTTPClient.Timeout != 0 {
		t.Fatalf("HTTPClient.Timeout = %v, want 0", c.HTTPClient.Timeout)
	}
}

func TestWithBackoff_DefaultsValuesIndependently(t *testing.T) {
	t.Parallel()

	t.Run("defaults initial only", func(t *testing.T) {
		t.Parallel()

		c := &client.Client{}

		if err := client.WithBackoff(0, 2*time.Second)(c); err != nil {
			t.Fatalf("WithBackoff() error = %v", err)
		}

		if c.InitialBackoff != 400*time.Millisecond {
			t.Fatalf("InitialBackoff = %v, want 400ms", c.InitialBackoff)
		}
		if c.MaxBackoff != 2*time.Second {
			t.Fatalf("MaxBackoff = %v, want 2s", c.MaxBackoff)
		}
	})

	t.Run("defaults max only", func(t *testing.T) {
		t.Parallel()

		c := &client.Client{}

		if err := client.WithBackoff(time.Second, 0)(c); err != nil {
			t.Fatalf("WithBackoff() error = %v", err)
		}

		if c.InitialBackoff != time.Second {
			t.Fatalf("InitialBackoff = %v, want 1s", c.InitialBackoff)
		}
		if c.MaxBackoff != 5*time.Second {
			t.Fatalf("MaxBackoff = %v, want 5s", c.MaxBackoff)
		}
	})
}

func TestWithHTTPTimeout_CreatesHTTPClientWhenNil(t *testing.T) {
	t.Parallel()

	c := &client.Client{
		HTTPClient: nil,
	}

	opt := client.WithHTTPTimeout(1500 * time.Millisecond)
	if err := opt(c); err != nil {
		t.Fatalf("WithHTTPTimeout() error = %v", err)
	}

	if c.HTTPClient == nil {
		t.Fatal("HTTPClient = nil, want non-nil")
	}
	if c.HTTPClient.Timeout != 1500*time.Millisecond {
		t.Fatalf("HTTPClient.Timeout = %v, want %v", c.HTTPClient.Timeout, 1500*time.Millisecond)
	}
}

func TestWithHTTPTimeout_UsesExistingHTTPClient(t *testing.T) {
	t.Parallel()

	hc := &http.Client{Timeout: 5 * time.Second}
	c := &client.Client{
		HTTPClient: hc,
	}

	opt := client.WithHTTPTimeout(2 * time.Second)
	if err := opt(c); err != nil {
		t.Fatalf("WithHTTPTimeout() error = %v", err)
	}

	if c.HTTPClient != hc {
		t.Fatal("HTTPClient pointer was replaced, want same client")
	}
	if c.HTTPClient.Timeout != 2*time.Second {
		t.Fatalf("HTTPClient.Timeout = %v, want %v", c.HTTPClient.Timeout, 2*time.Second)
	}
}

func TestWithHTTPTimeout_ErrorWhenNegative(t *testing.T) {
	t.Parallel()

	hc := &http.Client{Timeout: 3 * time.Second}
	c := &client.Client{
		HTTPClient: hc,
	}

	opt := client.WithHTTPTimeout(-1 * time.Second)
	err := opt(c)
	if err == nil {
		t.Fatal("WithHTTPTimeout() error = nil, want error")
	}
	if err.Error() != "http timeout cannot be negative" {
		t.Fatalf("error = %q, want %q", err.Error(), "http timeout cannot be negative")
	}
	if c.HTTPClient != hc {
		t.Fatal("HTTPClient pointer changed on error")
	}
	if c.HTTPClient.Timeout != 3*time.Second {
		t.Fatalf("HTTPClient.Timeout = %v, want %v", c.HTTPClient.Timeout, 3*time.Second)
	}
}

func TestWithMaxRetries_NegativeNormalizesToZero(t *testing.T) {
	t.Parallel()

	c := &client.Client{
		MaxRetries: 7,
	}

	opt := client.WithMaxRetries(-3)
	if err := opt(c); err != nil {
		t.Fatalf("WithMaxRetries() error = %v", err)
	}

	if c.MaxRetries != 0 {
		t.Fatalf("MaxRetries = %d, want %d", c.MaxRetries, 0)
	}
}

func TestWithBackoff_DefaultsWhenInitialAndMaxAreZero(t *testing.T) {
	t.Parallel()

	c := &client.Client{}

	opt := client.WithBackoff(0, 0)
	if err := opt(c); err != nil {
		t.Fatalf("WithBackoff() error = %v", err)
	}

	if c.InitialBackoff != 400*time.Millisecond {
		t.Fatalf("InitialBackoff = %v, want %v", c.InitialBackoff, 400*time.Millisecond)
	}
	if c.MaxBackoff != 5*time.Second {
		t.Fatalf("MaxBackoff = %v, want %v", c.MaxBackoff, 5*time.Second)
	}
}

func TestWithBackoff_DefaultsWhenValuesAreNegative(t *testing.T) {
	t.Parallel()

	c := &client.Client{}

	opt := client.WithBackoff(-1*time.Second, -2*time.Second)
	if err := opt(c); err != nil {
		t.Fatalf("WithBackoff() error = %v", err)
	}

	if c.InitialBackoff != 400*time.Millisecond {
		t.Fatalf("InitialBackoff = %v, want %v", c.InitialBackoff, 400*time.Millisecond)
	}
	if c.MaxBackoff != 5*time.Second {
		t.Fatalf("MaxBackoff = %v, want %v", c.MaxBackoff, 5*time.Second)
	}
}

func TestWithBackoff_PromotesMaxToInitialWhenMaxLessThanInitial(t *testing.T) {
	t.Parallel()

	c := &client.Client{}

	initial := 3 * time.Second
	max := 1 * time.Second

	opt := client.WithBackoff(initial, max)
	if err := opt(c); err != nil {
		t.Fatalf("WithBackoff() error = %v", err)
	}

	if c.InitialBackoff != initial {
		t.Fatalf("InitialBackoff = %v, want %v", c.InitialBackoff, initial)
	}
	if c.MaxBackoff != initial {
		t.Fatalf("MaxBackoff = %v, want %v", c.MaxBackoff, initial)
	}
}

func TestWithPollWait_DefaultsWhenInitialAndMaxAreZero(t *testing.T) {
	t.Parallel()

	c := &client.Client{}

	opt := client.WithPollWait(0, 0)
	if err := opt(c); err != nil {
		t.Fatalf("WithPollWait() error = %v", err)
	}

	if c.PollInitialWait != 1*time.Second {
		t.Fatalf("PollInitialWait = %v, want %v", c.PollInitialWait, 1*time.Second)
	}
	if c.PollMaxWait != 120*time.Second {
		t.Fatalf("PollMaxWait = %v, want %v", c.PollMaxWait, 120*time.Second)
	}
}

func TestWithPollWait_DefaultsWhenValuesAreNegative(t *testing.T) {
	t.Parallel()

	c := &client.Client{}

	opt := client.WithPollWait(-1*time.Second, -2*time.Second)
	if err := opt(c); err != nil {
		t.Fatalf("WithPollWait() error = %v", err)
	}

	if c.PollInitialWait != 1*time.Second {
		t.Fatalf("PollInitialWait = %v, want %v", c.PollInitialWait, 1*time.Second)
	}
	if c.PollMaxWait != 120*time.Second {
		t.Fatalf("PollMaxWait = %v, want %v", c.PollMaxWait, 120*time.Second)
	}
}

func TestWithPollWait_PromotesMaxToInitialWhenMaxLessThanInitial(t *testing.T) {
	t.Parallel()

	c := &client.Client{}

	initial := 10 * time.Second
	max := 3 * time.Second

	opt := client.WithPollWait(initial, max)
	if err := opt(c); err != nil {
		t.Fatalf("WithPollWait() error = %v", err)
	}

	if c.PollInitialWait != initial {
		t.Fatalf("PollInitialWait = %v, want %v", c.PollInitialWait, initial)
	}
	if c.PollMaxWait != initial {
		t.Fatalf("PollMaxWait = %v, want %v", c.PollMaxWait, initial)
	}
}
