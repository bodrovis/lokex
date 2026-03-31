package retry_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bodrovis/lokex/v2/client/internal/retry"
)

func TestWithExpBackoff(t *testing.T) {
	t.Parallel()

	t.Run("context error before attempt is returned", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		called := false
		err := retry.WithExpBackoff(
			ctx,
			"download bundle",
			2,
			time.Millisecond,
			10*time.Millisecond,
			func(_ int) error {
				called = true
				return nil
			},
			func(error) bool { return true },
		)
		if err == nil {
			t.Fatal("WithExpBackoff() error = nil, want non-nil")
		}
		if err.Error() != "download bundle (attempt 1/3): context: context canceled" {
			t.Fatalf(
				"error = %q, want %q",
				err.Error(),
				"download bundle (attempt 1/3): context: context canceled",
			)
		}
		if called {
			t.Fatal("op was called, want it not to be called when context is already canceled")
		}
	})
}

func TestResolveRetryable(t *testing.T) {
	t.Parallel()

	t.Run("returns provided function when non nil", func(t *testing.T) {
		t.Parallel()

		wantCalled := false
		custom := func(err error) bool {
			wantCalled = true
			return err != nil
		}

		got := retry.ExportResolveRetryable(custom)
		if got == nil {
			t.Fatal("ResolveRetryable() = nil, want non-nil")
		}

		ok := got(errors.New("boom"))
		if !ok {
			t.Fatal("resolved retryable func returned false, want true")
		}
		if !wantCalled {
			t.Fatal("provided retryable func was not called")
		}
	})

	t.Run("returns default function when nil", func(t *testing.T) {
		t.Parallel()

		got := retry.ExportResolveRetryable(nil)
		if got == nil {
			t.Fatal("ResolveRetryable() = nil, want non-nil")
		}
	})
}

func TestComputeRetryDelay(t *testing.T) {
	tests := []struct {
		name       string
		backoff    time.Duration
		maxBackoff time.Duration
		mockJitter func(time.Duration) time.Duration
		check      func(t *testing.T, got time.Duration)
	}{
		{
			name:       "non positive jitter result falls back to millisecond",
			backoff:    time.Second,
			maxBackoff: time.Hour,
			mockJitter: func(time.Duration) time.Duration {
				return 0
			},
			check: func(t *testing.T, got time.Duration) {
				t.Helper()

				if got != time.Millisecond {
					t.Fatalf("got = %v, want %v", got, time.Millisecond)
				}
			},
		},
		{
			name:       "delay is capped by max backoff",
			backoff:    time.Hour,
			maxBackoff: time.Millisecond,
			check: func(t *testing.T, got time.Duration) {
				t.Helper()

				if got != time.Millisecond {
					t.Fatalf("got = %v, want %v", got, time.Millisecond)
				}
			},
		},
		{
			name:       "delay does not exceed max backoff",
			backoff:    time.Millisecond,
			maxBackoff: time.Second,
			check: func(t *testing.T, got time.Duration) {
				t.Helper()

				if got <= 0 {
					t.Fatalf("got = %v, want > 0", got)
				}
				if got > time.Second {
					t.Fatalf("got = %v, want <= %v", got, time.Second)
				}
			},
		},
		{
			name:       "zero max backoff returns zero after cap",
			backoff:    time.Millisecond,
			maxBackoff: 0,
			check: func(t *testing.T, got time.Duration) {
				t.Helper()

				if got != 0 {
					t.Fatalf("got = %v, want %v", got, time.Duration(0))
				}
			},
		},
		{
			name:       "jitter result larger than max is capped",
			backoff:    time.Second,
			maxBackoff: time.Second,
			mockJitter: func(time.Duration) time.Duration {
				return 5 * time.Second
			},
			check: func(t *testing.T, got time.Duration) {
				t.Helper()

				if got != time.Second {
					t.Fatalf("got = %v, want %v", got, time.Second)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mockJitter != nil {
				restore := retry.ExportSetJitteredBackoffForTest(tt.mockJitter)
				defer restore()
			}

			got := retry.ExportComputeRetryDelay(tt.backoff, tt.maxBackoff)
			tt.check(t, got)
		})
	}
}

func TestWrapErr(t *testing.T) {
	t.Parallel()

	baseErr := errors.New("boom")

	t.Run("empty label returns original error", func(t *testing.T) {
		t.Parallel()

		got := retry.ExportWrapErr("", 0, 3, baseErr)
		if !errors.Is(got, baseErr) {
			t.Fatal("WrapErr() does not wrap original error")
		}
		if got.Error() != "boom" {
			t.Fatalf("error = %q, want %q", got.Error(), "boom")
		}
	})

	t.Run("label wraps error with attempt info", func(t *testing.T) {
		t.Parallel()

		got := retry.ExportWrapErr("download bundle", 1, 3, baseErr)
		if !errors.Is(got, baseErr) {
			t.Fatal("WrapErr() does not wrap original error")
		}
		if got.Error() != "download bundle (attempt 2/3): boom" {
			t.Fatalf(
				"error = %q, want %q",
				got.Error(),
				"download bundle (attempt 2/3): boom",
			)
		}
	})
}

func TestWrapCtxErr(t *testing.T) {
	t.Parallel()

	baseErr := context.Canceled

	t.Run("empty label returns original error", func(t *testing.T) {
		t.Parallel()

		got := retry.ExportWrapCtxErr("", 0, 3, baseErr)
		if !errors.Is(got, baseErr) {
			t.Fatal("WrapCtxErr() does not wrap original error")
		}
		if got.Error() != "context canceled" {
			t.Fatalf("error = %q, want %q", got.Error(), "context canceled")
		}
	})

	t.Run("label wraps context error with attempt info", func(t *testing.T) {
		t.Parallel()

		got := retry.ExportWrapCtxErr("download bundle", 1, 3, baseErr)
		if !errors.Is(got, baseErr) {
			t.Fatal("WrapCtxErr() does not wrap original error")
		}
		if got.Error() != "download bundle (attempt 2/3): context: context canceled" {
			t.Fatalf(
				"error = %q, want %q",
				got.Error(),
				"download bundle (attempt 2/3): context: context canceled",
			)
		}
	})
}
