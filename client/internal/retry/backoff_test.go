package retry_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"testing/synctest"
	"time"

	"github.com/bodrovis/lokex/v2/client/internal/retry"
	"github.com/bodrovis/lokex/v2/internal/apierr"
)

func TestWithExpBackoff(t *testing.T) {
	t.Parallel()

	t.Run("succeeds on first attempt", func(t *testing.T) {
		calls := 0

		err := retry.WithExpBackoff(
			context.Background(),
			"",
			3,
			time.Millisecond,
			time.Millisecond,
			func(attempt int) error {
				calls++

				if attempt != 0 {
					t.Fatalf("attempt = %d, want 0", attempt)
				}

				return nil
			},
			func(error) bool { return true },
		)

		if err != nil {
			t.Fatalf("WithExpBackoff() error = %v", err)
		}
		if calls != 1 {
			t.Fatalf("calls = %d, want 1", calls)
		}
	})

	t.Run("retries retryable error and succeeds", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			retryErr := errors.New("temporary failure")
			var attempts []int

			err := retry.WithExpBackoff(
				t.Context(),
				"",
				3,
				100*time.Millisecond,
				time.Second,
				func(attempt int) error {
					attempts = append(attempts, attempt)

					if attempt < 2 {
						return retryErr
					}

					return nil
				},
				func(err error) bool {
					return errors.Is(err, retryErr)
				},
			)

			if err != nil {
				t.Fatalf("WithExpBackoff() error = %v", err)
			}

			want := []int{0, 1, 2}
			if len(attempts) != len(want) {
				t.Fatalf("attempts = %v, want %v", attempts, want)
			}

			for i := range want {
				if attempts[i] != want[i] {
					t.Fatalf("attempts = %v, want %v", attempts, want)
				}
			}
		})
	})

	t.Run("non retryable error stops immediately", func(t *testing.T) {
		baseErr := errors.New("bad request")
		calls := 0

		err := retry.WithExpBackoff(
			context.Background(),
			"",
			3,
			time.Millisecond,
			time.Second,
			func(attempt int) error {
				calls++
				return baseErr
			},
			func(error) bool { return false },
		)

		if !errors.Is(err, baseErr) {
			t.Fatalf("error = %v, want %v", err, baseErr)
		}
		if err != baseErr {
			t.Fatalf("error = %v, want original error", err)
		}
		if calls != 1 {
			t.Fatalf("calls = %d, want 1", calls)
		}
	})

	t.Run("stops after max retries", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			baseErr := errors.New("temporary failure")
			calls := 0

			err := retry.WithExpBackoff(
				t.Context(),
				"download bundle",
				2,
				100*time.Millisecond,
				time.Second,
				func(attempt int) error {
					calls++
					return baseErr
				},
				func(error) bool { return true },
			)

			if !errors.Is(err, baseErr) {
				t.Fatalf("error does not wrap base error: %v", err)
			}

			want := "download bundle (attempt 3/3): temporary failure"
			if err.Error() != want {
				t.Fatalf("error = %q, want %q", err.Error(), want)
			}

			if calls != 3 {
				t.Fatalf("calls = %d, want 3", calls)
			}
		})
	})

	t.Run("uses default retryable classifier when nil", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			calls := 0

			err := retry.WithExpBackoff(
				t.Context(),
				"",
				1,
				100*time.Millisecond,
				time.Second,
				func(attempt int) error {
					calls++

					if attempt == 0 {
						return &apierr.APIError{
							Status: http.StatusServiceUnavailable,
						}
					}

					return nil
				},
				nil,
			)

			if err != nil {
				t.Fatalf("WithExpBackoff() error = %v", err)
			}
			if calls != 2 {
				t.Fatalf("calls = %d, want 2", calls)
			}
		})
	})

	t.Run("context error before attempt is returned", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		called := false

		err := retry.WithExpBackoff(
			ctx,
			"download bundle",
			2,
			time.Millisecond,
			10*time.Millisecond,
			func(int) error {
				called = true
				return nil
			},
			func(error) bool { return true },
		)

		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context.Canceled", err)
		}

		want := "download bundle (attempt 1/3): context: context canceled"
		if err.Error() != want {
			t.Fatalf("error = %q, want %q", err.Error(), want)
		}

		if called {
			t.Fatal("op was called, want it not to be called")
		}
	})

	t.Run("context deadline during backoff is returned", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(
				t.Context(),
				10*time.Millisecond,
			)
			defer cancel()

			calls := 0

			err := retry.WithExpBackoff(
				ctx,
				"download bundle",
				2,
				time.Second,
				time.Second,
				func(int) error {
					calls++
					return errors.New("temporary failure")
				},
				func(error) bool { return true },
			)

			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf(
					"error = %v, want context.DeadlineExceeded",
					err,
				)
			}

			want := "download bundle (attempt 1/3): context: context deadline exceeded"
			if err.Error() != want {
				t.Fatalf("error = %q, want %q", err.Error(), want)
			}

			if calls != 1 {
				t.Fatalf("calls = %d, want 1", calls)
			}
		})
	})

	t.Run("delay is capped by max backoff", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			start := time.Now()
			calls := 0

			err := retry.WithExpBackoff(
				t.Context(),
				"",
				1,
				time.Second,
				time.Millisecond,
				func(attempt int) error {
					calls++

					if attempt == 0 {
						return errors.New("temporary failure")
					}

					if elapsed := time.Since(start); elapsed != time.Millisecond {
						t.Fatalf(
							"second attempt after %v, want %v",
							elapsed,
							time.Millisecond,
						)
					}

					return nil
				},
				func(error) bool { return true },
			)

			if err != nil {
				t.Fatalf("WithExpBackoff() error = %v", err)
			}
			if calls != 2 {
				t.Fatalf("calls = %d, want 2", calls)
			}
		})
	})
}
