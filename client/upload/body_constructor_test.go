package upload_test

import (
	"context"
	"errors"
	"io"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bodrovis/lokex/v2/client/upload"
)

type firstNilThenCanceledContext struct {
	done  chan struct{}
	calls atomic.Int32
}

func newFirstNilThenCanceledContext() *firstNilThenCanceledContext {
	done := make(chan struct{})
	close(done)

	return &firstNilThenCanceledContext{
		done: done,
	}
}

func (c *firstNilThenCanceledContext) Deadline() (time.Time, bool) {
	return time.Time{}, false
}

func (c *firstNilThenCanceledContext) Done() <-chan struct{} {
	return c.done
}

func (c *firstNilThenCanceledContext) Err() error {
	if c.calls.Add(1) == 1 {
		return nil
	}
	return context.Canceled
}

func (c *firstNilThenCanceledContext) Value(key any) any {
	return nil
}

func TestNewUploadBody(t *testing.T) {
	t.Run("nil context uses background", func(t *testing.T) {
		t.Parallel()

		rc, err := upload.ExportNewUploadBody(
			//lint:ignore SA1012 intentionally passing nil context in this test
			nil, //nolint:staticcheck // nil ctx is required for this test
			upload.UploadParams{
				"data": "dGVzdA==",
			},
			"",
		)
		if err != nil {
			t.Fatalf("NewUploadBody() unexpected error = %v", err)
		}
		if rc == nil {
			t.Fatal("reader = nil, want non-nil")
		}
		defer func() {
			_ = rc.Close()
		}()

		_, _ = io.ReadAll(rc)
	})

	t.Run("canceled context returns error before pipe creation", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		rc, err := upload.ExportNewUploadBody(
			ctx,
			upload.UploadParams{
				"data": "dGVzdA==",
			},
			"",
		)
		if err == nil {
			t.Fatal("NewUploadBody() error = nil, want non-nil")
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want %v", err, context.Canceled)
		}
		if rc != nil {
			t.Fatal("reader != nil, want nil on error")
		}
	})

	t.Run("writer bails early when context is already canceled inside goroutine", func(t *testing.T) {
		ctx := newFirstNilThenCanceledContext()

		rc, err := upload.ExportNewUploadBody(
			ctx,
			upload.UploadParams{
				"data": "dGVzdA==",
			},
			"",
		)
		if err != nil {
			t.Fatalf("NewUploadBody() unexpected error = %v", err)
		}
		if rc == nil {
			t.Fatal("reader = nil, want non-nil")
		}
		defer func() {
			_ = rc.Close()
		}()

		_, err = io.ReadAll(rc)
		if err == nil {
			t.Fatal("ReadAll() error = nil, want non-nil")
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want %v", err, context.Canceled)
		}
	})
}
