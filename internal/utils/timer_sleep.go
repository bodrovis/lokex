package utils

import (
	"context"
	"time"
)

// sleepWithTimer waits for d or returns early on ctx cancellation.
// Timer is reused to avoid allocations.
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
