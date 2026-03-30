package utils_test

import (
	"context"
	"testing"
	"time"

	"github.com/bodrovis/lokex/v2/internal/utils"
)

func TestSleepWithTimer_UsesDefaultDelayWhenDurationNonPositive(t *testing.T) {
	t.Parallel()

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
			d:    -1 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			timer := time.NewTimer(time.Hour)
			defer timer.Stop()

			start := time.Now()
			err := utils.SleepWithTimer(ctx, timer, tt.d)
			elapsed := time.Since(start)

			if err != nil {
				t.Fatalf("SleepWithTimer() error = %v", err)
			}

			if elapsed < 8*time.Millisecond {
				t.Fatalf("elapsed = %v, want at least about 10ms", elapsed)
			}
		})
	}
}
