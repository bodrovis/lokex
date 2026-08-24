package utils_test

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/bodrovis/lokex/v2/internal/utils"
)

func TestSleepWithTimer_UsesDefaultDelayWhenDurationNonPositive(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
	}{
		{
			name: "zero duration",
			d:    0,
		},
		{
			name: "negative duration",
			d:    -time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				timer := time.NewTimer(time.Hour)
				defer timer.Stop()

				start := time.Now()

				err := utils.SleepWithTimer(t.Context(), timer, tt.d)
				if err != nil {
					t.Fatalf("SleepWithTimer() error = %v", err)
				}

				if elapsed := time.Since(start); elapsed != 10*time.Millisecond {
					t.Fatalf("elapsed = %v, want %v", elapsed, 10*time.Millisecond)
				}
			})
		})
	}
}

func TestSleepWithTimer_ReturnsContextErrorWhenCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	timer := time.NewTimer(time.Hour)
	defer timer.Stop()

	err := utils.SleepWithTimer(ctx, timer, time.Second)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("SleepWithTimer() error = %v, want context.Canceled", err)
	}
}

func TestSleepWithTimer_ReusesExpiredTimer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		timer := time.NewTimer(time.Millisecond)
		defer timer.Stop()

		// Let the original timer expire without receiving from timer.C.
		synctest.Sleep(20 * time.Millisecond)

		start := time.Now()

		err := utils.SleepWithTimer(
			t.Context(),
			timer,
			20*time.Millisecond,
		)
		if err != nil {
			t.Fatalf("SleepWithTimer() error = %v", err)
		}

		if elapsed := time.Since(start); elapsed != 20*time.Millisecond {
			t.Fatalf("elapsed = %v, want %v", elapsed, 20*time.Millisecond)
		}
	})
}
