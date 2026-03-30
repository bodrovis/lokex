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
