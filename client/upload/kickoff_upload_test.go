package upload_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/bodrovis/lokex/v2/client"
	"github.com/bodrovis/lokex/v2/client/upload"
)

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

func TestUploadBodyFactory_Read(t *testing.T) {
	t.Parallel()

	n, err := upload.ExportUploadBodyFactoryReadForTest()
	if n != 0 {
		t.Fatalf("n = %d, want %d", n, 0)
	}
	if !errors.Is(err, io.EOF) {
		t.Fatalf("err = %v, want %v", err, io.EOF)
	}
}
