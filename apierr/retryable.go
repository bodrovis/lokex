package apierr

import (
	"errors"
	"io"
	"math/rand"
	"net/http"
	"syscall"
	"time"
)

// IsRetryable says "worth another shot?" (backoff still on the caller).
func IsRetryable(err error) bool {
	// timeouts from net/http, http2, tls, etc.
	var to interface{ Timeout() bool }
	if errors.As(err, &to) && to.Timeout() {
		return true
	}

	// flaky connections / short reads
	if errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrClosedPipe) ||
		errors.Is(err, syscall.ECONNRESET) {
		return true
	}

	// server returned non-2xx
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		switch apiErr.Status {
		case http.StatusRequestTimeout, // 408
			http.StatusTooEarly,            // 425
			http.StatusTooManyRequests,     // 429
			http.StatusInternalServerError, // 500
			http.StatusBadGateway,          // 502
			http.StatusServiceUnavailable,  // 503
			http.StatusGatewayTimeout:      // 504
			return true
		}
	}
	return false
}

// JitteredBackoff returns ~0.5x..1.5x of base with uniform jitter.
// If base <= 0, defaults to 300ms.
func JitteredBackoff(base time.Duration) time.Duration {
	if base <= 0 {
		base = 300 * time.Millisecond
	}
	// delta: [0, base)
	delta := time.Duration(rand.Int63n(int64(base)))
	return base/2 + delta
}
