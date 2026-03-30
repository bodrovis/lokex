package client_test

import (
	"testing"
	"time"

	"github.com/bodrovis/lokex/v2/client"
)

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
	if c.UserAgent != "lokex/2.0.0" {
		t.Fatalf("UserAgent = %q, want %q", c.UserAgent, "lokex/2.0.0")
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
