package retry

import (
	"context"
	"fmt"
	"time"

	"github.com/bodrovis/lokex/v2/internal/apierr"
	"github.com/bodrovis/lokex/v2/internal/utils"
)

var jitteredBackoff = apierr.JitteredBackoff

// WithExpBackoff runs op with retries using exponential backoff + jitter.
// MaxRetries is the number of retries after the initial attempt.
// If isRetryable is nil, apierr.IsRetryable is used.
// If ctx is canceled or its deadline is exceeded, ctx.Err() is returned
// wrapped with label context when label is provided.
func WithExpBackoff(
	ctx context.Context,
	label string,
	maxRetries int,
	initialBackoff time.Duration,
	maxBackoff time.Duration,
	op func(attempt int) error,
	isRetryable func(error) bool,
) error {
	isRetryable = resolveRetryable(isRetryable)

	totalAttempts := maxRetries + 1
	backoff := initialBackoff

	timer := newStoppedTimer()
	defer stopAndDrainTimer(timer)

	for attempt := 0; ; attempt++ {
		if err := contextAttemptErr(ctx, label, attempt, totalAttempts); err != nil {
			return err
		}

		err := op(attempt)
		if err == nil {
			return nil
		}

		if err := contextAttemptErr(ctx, label, attempt, totalAttempts); err != nil {
			return err
		}

		if shouldStopRetry(attempt, maxRetries, err, isRetryable) {
			return wrapErr(label, attempt, totalAttempts, err)
		}

		delay := computeRetryDelay(backoff, maxBackoff)
		if err := utils.SleepWithTimer(ctx, timer, delay); err != nil {
			return wrapCtxErr(label, attempt, totalAttempts, err)
		}

		backoff = nextBackoff(backoff, maxBackoff)
	}
}

func resolveRetryable(fn func(error) bool) func(error) bool {
	if fn != nil {
		return fn
	}
	return apierr.IsRetryable
}

func newStoppedTimer() *time.Timer {
	timer := time.NewTimer(time.Hour)
	stopAndDrainTimer(timer)
	return timer
}

func stopAndDrainTimer(timer *time.Timer) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}

func contextAttemptErr(
	ctx context.Context,
	label string,
	attempt int,
	totalAttempts int,
) error {
	if err := ctx.Err(); err != nil {
		return wrapCtxErr(label, attempt, totalAttempts, err)
	}
	return nil
}

func shouldStopRetry(
	attempt int,
	maxRetries int,
	err error,
	isRetryable func(error) bool,
) bool {
	return !isRetryable(err) || attempt >= maxRetries
}

func computeRetryDelay(backoff, maxBackoff time.Duration) time.Duration {
	delay := jitteredBackoff(backoff)
	if delay <= 0 {
		delay = time.Millisecond
	}
	if delay > maxBackoff {
		delay = maxBackoff
	}
	return delay
}

func nextBackoff(backoff, maxBackoff time.Duration) time.Duration {
	backoff *= 2
	if backoff > maxBackoff {
		return maxBackoff
	}
	return backoff
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
