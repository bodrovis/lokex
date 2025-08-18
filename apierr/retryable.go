package apierr

import (
	"context"
	"errors"
	"math/rand"
	"net/http"
	"time"
)

// IsRetryable says "worth another shot?" (backoff still on the caller).
func IsRetryable(err error) bool {
	// context issues, caller may retry with different timeout
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}

	var to interface{ Timeout() bool }
	if errors.As(err, &to) && to.Timeout() {
		return true
	}

	// server returned non-2xx
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		switch apiErr.Status {
		case http.StatusTooManyRequests, // 429
			http.StatusInternalServerError, // 500
			http.StatusBadGateway,          // 502
			http.StatusServiceUnavailable,  // 503
			http.StatusGatewayTimeout:      // 504
			return true
		}
	}
	return false
}

// IsRateLimited specifically checks 429.
func IsRateLimited(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Status == http.StatusTooManyRequests
}

func JitteredBackoff(base time.Duration) time.Duration {
	// 0.5x .. 1.5x jitter
	if base <= 0 {
		base = 300 * time.Millisecond
	}
	delta := time.Duration(rand.Int63n(int64(base))) // 0..base-1
	return base/2 + delta
}
