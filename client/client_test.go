package client_test

import (
	"testing"

	"github.com/bodrovis/lokex/client"
)

func TestNewClient_DefaultBaseURL(t *testing.T) {
	token := "tok123"
	projectID := "proj456"

	c, err := client.NewClient(token, projectID, nil)

	if err != nil {
		t.Fatalf("Cannot create client")
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
}

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
