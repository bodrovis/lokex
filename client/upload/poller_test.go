package upload_test

import (
	"context"
	"testing"

	"github.com/bodrovis/lokex/v2/client"
	"github.com/bodrovis/lokex/v2/client/internal/background"
	"github.com/bodrovis/lokex/v2/client/upload"
)

func TestPollUntilFinished(t *testing.T) {
	t.Run("empty process id", func(t *testing.T) {
		t.Parallel()

		c, err := client.NewClient("123", "abc")
		if err != nil {
			t.Fatal(err)
		}
		u := upload.NewUploader(c)

		got, err := upload.ExportPollUntilFinished(u, context.Background(), "   \t\n  ")
		if err == nil {
			t.Fatal("PollUntilFinished() error = nil, want non-nil")
		}
		if err.Error() != "upload: empty process_id" {
			t.Fatalf("error = %q, want %q", err.Error(), "upload: empty process_id")
		}
		if got != "" {
			t.Fatalf("got = %q, want empty string on error", got)
		}
	})

	t.Run("no process results returned", func(t *testing.T) {
		restore := upload.ExportSetPollProcessesForTest(
			func(context.Context, []string, *client.Client) ([]background.QueuedProcess, error) {
				return []background.QueuedProcess{}, nil
			},
		)
		defer restore()

		c, err := client.NewClient("123", "abc")
		if err != nil {
			t.Fatal(err)
		}
		u := upload.NewUploader(c)

		got, err := upload.ExportPollUntilFinished(u, context.Background(), "pid-1")
		if err == nil {
			t.Fatal("PollUntilFinished() error = nil, want non-nil")
		}
		if err.Error() != "upload: no process results returned (process_id=pid-1)" {
			t.Fatalf(
				"error = %q, want %q",
				err.Error(),
				"upload: no process results returned (process_id=pid-1)",
			)
		}
		if got != "" {
			t.Fatalf("got = %q, want empty string on error", got)
		}
	})

	t.Run("failed process with message", func(t *testing.T) {
		restore := upload.ExportSetPollProcessesForTest(
			func(context.Context, []string, *client.Client) ([]background.QueuedProcess, error) {
				return []background.QueuedProcess{
					{
						ProcessID: "pid-1",
						Status:    background.StatusFailed,
						Message:   " bad format ",
					},
				}, nil
			},
		)
		defer restore()

		c, err := client.NewClient("123", "abc")
		if err != nil {
			t.Fatal(err)
		}
		u := upload.NewUploader(c)

		got, err := upload.ExportPollUntilFinished(u, context.Background(), "pid-1")
		if err == nil {
			t.Fatal("PollUntilFinished() error = nil, want non-nil")
		}
		if err.Error() != "upload: process pid-1 failed: bad format" {
			t.Fatalf(
				"error = %q, want %q",
				err.Error(),
				"upload: process pid-1 failed: bad format",
			)
		}
		if got != "" {
			t.Fatalf("got = %q, want empty string on error", got)
		}
	})

	t.Run("failed process without message", func(t *testing.T) {
		restore := upload.ExportSetPollProcessesForTest(
			func(context.Context, []string, *client.Client) ([]background.QueuedProcess, error) {
				return []background.QueuedProcess{
					{
						ProcessID: "pid-1",
						Status:    background.StatusFailed,
					},
				}, nil
			},
		)
		defer restore()

		c, err := client.NewClient("123", "abc")
		if err != nil {
			t.Fatal(err)
		}
		u := upload.NewUploader(c)

		got, err := upload.ExportPollUntilFinished(u, context.Background(), "pid-1")
		if err == nil {
			t.Fatal("PollUntilFinished() error = nil, want non-nil")
		}
		if err.Error() != "upload: process pid-1 failed" {
			t.Fatalf("error = %q, want %q", err.Error(), "upload: process pid-1 failed")
		}
		if got != "" {
			t.Fatalf("got = %q, want empty string on error", got)
		}
	})

	t.Run("non terminal status returns did not finish", func(t *testing.T) {
		restore := upload.ExportSetPollProcessesForTest(
			func(context.Context, []string, *client.Client) ([]background.QueuedProcess, error) {
				return []background.QueuedProcess{
					{
						ProcessID: "pid-1",
						Status:    "queued \n",
					},
				}, nil
			},
		)
		defer restore()

		c, err := client.NewClient("123", "abc")
		if err != nil {
			t.Fatal(err)
		}
		u := upload.NewUploader(c)

		got, err := upload.ExportPollUntilFinished(u, context.Background(), "pid-1")
		if err == nil {
			t.Fatal("PollUntilFinished() error = nil, want non-nil")
		}
		if err.Error() != `upload: process pid-1 did not finish (status="queued")` {
			t.Fatalf(
				"error = %q, want %q",
				err.Error(),
				`upload: process pid-1 did not finish (status="queued")`,
			)
		}
		if got != "" {
			t.Fatalf("got = %q, want empty string on error", got)
		}
	})
}
