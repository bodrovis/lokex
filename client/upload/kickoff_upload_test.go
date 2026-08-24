package upload_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/bodrovis/lokex/v2/client"
	"github.com/bodrovis/lokex/v2/client/upload"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestKickoffUploadStreaming(t *testing.T) {
	t.Run("nil uploader", func(t *testing.T) {
		t.Parallel()

		var u *upload.Uploader

		got, err := upload.ExportKickoffUploadStreaming(
			u,
			context.Background(),
			upload.UploadParams{"filename": "test.json"},
			"/tmp/test.json",
		)
		if err == nil {
			t.Fatal("KickoffUploadStreaming() error = nil, want non-nil")
		}
		if err.Error() != "upload: kickoff: uploader/client is nil" {
			t.Fatalf("error = %q, want %q", err.Error(), "upload: kickoff: uploader/client is nil")
		}
		if got != "" {
			t.Fatalf("got = %q, want empty string on error", got)
		}
	})

	t.Run("nil client", func(t *testing.T) {
		t.Parallel()

		u := upload.ExportNewUploaderWithClientForTest(nil)

		got, err := upload.ExportKickoffUploadStreaming(
			u,
			context.Background(),
			upload.UploadParams{"filename": "test.json"},
			"/tmp/test.json",
		)
		if err == nil {
			t.Fatal("KickoffUploadStreaming() error = nil, want non-nil")
		}
		if err.Error() != "upload: kickoff: uploader/client is nil" {
			t.Fatalf("error = %q, want %q", err.Error(), "upload: kickoff: uploader/client is nil")
		}
		if got != "" {
			t.Fatalf("got = %q, want empty string on error", got)
		}
	})

	t.Run("missing local file path and data", func(t *testing.T) {
		t.Parallel()

		c, err := client.NewClient("123", "abc")
		if err != nil {
			t.Fatal(err)
		}
		u := upload.NewUploader(c)

		got, err := upload.ExportKickoffUploadStreaming(
			u,
			context.Background(),
			upload.UploadParams{
				"filename": "test.json",
			},
			"   \t\n  ",
		)
		if err == nil {
			t.Fatal("KickoffUploadStreaming() error = nil, want non-nil")
		}
		if err.Error() != "upload: kickoff: missing local file path and 'data'" {
			t.Fatalf(
				"error = %q, want %q",
				err.Error(),
				"upload: kickoff: missing local file path and 'data'",
			)
		}
		if got != "" {
			t.Fatalf("got = %q, want empty string on error", got)
		}
	})
}

func TestKickoffUploadStreaming_NilContext(t *testing.T) {
	t.Parallel()

	hc := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(
					`{"process":{"process_id":"process123"}}`,
				)),
				Header:  make(http.Header),
				Request: req,
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

	u := upload.ExportNewUploaderWithClientForTest(c)

	got, err := upload.ExportKickoffUploadStreaming(
		u,
		nil,
		upload.UploadParams{
			"filename": "test.json",
			"data":     "dGVzdA==",
		},
		"",
	)
	if err != nil {
		t.Fatalf("KickoffUploadStreaming() error = %v", err)
	}

	if got != "process123" {
		t.Fatalf("process ID = %q, want %q", got, "process123")
	}
}
