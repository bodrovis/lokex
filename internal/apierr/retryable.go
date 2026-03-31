// apierr/retryable.go
package apierr

import (
	"context"
	"errors"
	"io"
	"math/rand"
	"net"
	"net/http"
	"sync"
	"syscall"
	"time"
)

var (
	jitterRandMu sync.Mutex
	jitterRand   = rand.New(rand.NewSource(time.Now().UnixNano()))
)

// IsRetryable returns true only for transient failures.
// Order is IMPORTANT.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	if isNetTimeout(err) {
		return true
	}
	if isContextError(err) {
		return false
	}
	if hasTimeout(err) {
		return true
	}
	if isTransientIO(err) {
		return true
	}
	if isRetryableAPIError(err) {
		return true
	}

	return false
}

func isNetTimeout(err error) bool {
	var op *net.OpError
	return errors.As(err, &op) && op.Timeout()
}

func isContextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func hasTimeout(err error) bool {
	var te interface{ Timeout() bool }
	return errors.As(err, &te) && te.Timeout()
}

func isTransientIO(err error) bool {
	return errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrClosedPipe) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.EPIPE) ||
		errors.Is(err, syscall.ECONNABORTED)
}

func isRetryableAPIError(err error) bool {
	var ae *APIError
	if !errors.As(err, &ae) {
		return false
	}

	switch ae.Status {
	case http.StatusRequestTimeout,
		http.StatusTooEarly,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

// JitteredBackoff returns a randomized delay in [0.5*base, 1.5*base).
// If base <= 0, it falls back to 300ms.
//
// Note: we intentionally use a package-local PRNG guarded by a mutex.
// A *rand.Rand created via rand.New(...) is NOT goroutine-safe, so without
// the lock we'd get races when multiple retries happen concurrently.
func JitteredBackoff(base time.Duration) time.Duration {
	if base <= 0 {
		base = 300 * time.Millisecond
	}

	jitterRandMu.Lock()
	delta := time.Duration(jitterRand.Int63n(int64(base))) // [0, base)
	jitterRandMu.Unlock()

	return base/2 + delta
}
