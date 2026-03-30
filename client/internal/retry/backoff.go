package retry

import (
	"context"
	"fmt"
	"time"

	"github.com/bodrovis/lokex/v2/internal/apierr"
	"github.com/bodrovis/lokex/v2/internal/utils"
)

// WithExpBackoff runs op with retries using exponential backoff + jitter.
// MaxRetries is the number of *retries* after the initial attempt (total attempts = MaxRetries+1).
// If isRetryable is nil, apierr.IsRetryable is used.
// If ctx is canceled or its deadline is exceeded, ctx.Err() is returned (wrapped with label when provided).
func WithExpBackoff(
	ctx context.Context,
	label string,
	maxRetries int,
	initialBackoff time.Duration,
	maxBackoff time.Duration,
	op func(attempt int) error,
	isRetryable func(error) bool,
) error {
	if isRetryable == nil {
		isRetryable = apierr.IsRetryable
	}

	totalAttempts := maxRetries + 1

	// Reuse a single timer to avoid allocations on each retry.
	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	defer timer.Stop()

	for attempt := 0; ; attempt++ {
		// Bail fast if caller already canceled / deadline exceeded.
		if err := ctx.Err(); err != nil {
			return wrapCtxErr(label, attempt, totalAttempts, err)
		}

		err := op(attempt)
		if err == nil {
			return nil
		}

		// If ctx got canceled during the attempt, surface that cleanly.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return wrapCtxErr(label, attempt, totalAttempts, ctxErr)
		}

		// Not retryable or retries exhausted.
		if !isRetryable(err) || attempt >= maxRetries {
			return wrapErr(label, attempt, totalAttempts, err)
		}

		// Sleep with jittered backoff, capped.
		delay := apierr.JitteredBackoff(initialBackoff)
		if delay <= 0 {
			delay = time.Millisecond
		}
		if delay > maxBackoff {
			delay = maxBackoff
		}

		if err := utils.SleepWithTimer(ctx, timer, delay); err != nil {
			return wrapCtxErr(label, attempt, totalAttempts, err)
		}

		// Exponential growth capped.
		initialBackoff *= 2
		if initialBackoff > maxBackoff {
			initialBackoff = maxBackoff
		}
	}
}

func wrapErr(label string, attempt, total int, err error) error {
	if label == "" {
		return err
	}
	return fmt.Errorf("%s (attempt %d/%d): %w", label, attempt+1, total, err)
}

func wrapCtxErr(label string, attempt, total int, err error) error {
	if label == "" {
		return err
	}
	return fmt.Errorf("%s (attempt %d/%d): context: %w", label, attempt+1, total, err)
}
