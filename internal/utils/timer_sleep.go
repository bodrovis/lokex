package utils

import (
	"context"
	"time"
)

// SleepWithTimer waits for d or returns early on ctx cancellation.
// timer must be non-nil and must not be used concurrently.
func SleepWithTimer(ctx context.Context, timer *time.Timer, d time.Duration) error {
	if d <= 0 {
		d = 10 * time.Millisecond
	}
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(d)

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
