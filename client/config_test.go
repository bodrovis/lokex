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
	// invalid base url
	if _, err := client.NewClient("t", "p", client.WithBaseURL(":// nope")); err == nil {
		t.Fatalf("expected error for invalid base URL")
	}
	// WithHTTPClient(nil) should error
	if _, err := client.NewClient("t", "p", client.WithHTTPClient(nil)); err == nil {
		t.Fatalf("expected error for nil http client")
	}
	// trailing slash is enforced by WithBaseURL
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	baseNoSlash := srv.URL // no trailing slash
	c, err := client.NewClient("t", "p", client.WithBaseURL(baseNoSlash))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if got := c.BaseURL[len(c.BaseURL)-1:]; got != "/" {
		t.Fatalf("expected trailing slash, got %q", c.BaseURL)
	}
}

func TestWithBaseURL_ErrorWhenEmptyAfterTrim(t *testing.T) {
	t.Parallel()

	c := &client.Client{}

	opt := client.WithBaseURL("   \t\n   ")
	err := opt(c)
	if err == nil {
		t.Fatal("WithBaseURL() error = nil, want error")
	}
	if err.Error() != "base URL cannot be empty" {
		t.Fatalf("error = %q, want %q", err.Error(), "base URL cannot be empty")
	}
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
