package apierr_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/bodrovis/lokex/apierr"
)

// mock net.Error
type mockNetErr struct {
	msg     string
	timeout bool
}

func (m mockNetErr) Error() string { return m.msg }
func (m mockNetErr) Timeout() bool { return m.timeout }

func TestIsRetryable_ContextErrors(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"deadline exceeded", context.DeadlineExceeded, true},
		{"canceled", context.Canceled, true},
		{"wrapped deadline", fmt.Errorf("wrap: %w", context.DeadlineExceeded), true},
		{"wrapped canceled", fmt.Errorf("wrap: %w", context.Canceled), true},
		{"nil", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := apierr.IsRetryable(tc.err)
			if got != tc.want {
				t.Fatalf("IsRetryable(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestIsRetryable_NetError(t *testing.T) {
	timeoutErr := mockNetErr{msg: "i/o timeout", timeout: true}
	nonTimeoutErr := mockNetErr{msg: "conn refused", timeout: false}

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"net timeout", timeoutErr, true},
		{"wrapped net timeout", fmt.Errorf("wrap: %w", timeoutErr), true},
		{"net non-timeout", nonTimeoutErr, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := apierr.IsRetryable(tc.err)
			if got != tc.want {
				t.Fatalf("IsRetryable(%T) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestIsRetryable_APIStatuses(t *testing.T) {
	retryables := []int{
		http.StatusTooManyRequests,     // 429
		http.StatusInternalServerError, // 500
		http.StatusBadGateway,          // 502
		http.StatusServiceUnavailable,  // 503
		http.StatusGatewayTimeout,      // 504
	}
	for _, st := range retryables {
		t.Run(fmt.Sprintf("status_%d_retryable", st), func(t *testing.T) {
			err := &apierr.APIError{Status: st, Message: "boom"}
			if !apierr.IsRetryable(err) {
				t.Fatalf("IsRetryable(%d) = false, want true", st)
			}
			// wrapped
			if !apierr.IsRetryable(fmt.Errorf("wrap: %w", err)) {
				t.Fatalf("IsRetryable(wrapped %d) = false, want true", st)
			}
		})
	}

	nonRetryables := []int{
		http.StatusBadRequest,   // 400
		http.StatusUnauthorized, // 401
		http.StatusForbidden,    // 403
		http.StatusNotFound,     // 404
		418,
	}
	for _, st := range nonRetryables {
		t.Run(fmt.Sprintf("status_%d_nonretryable", st), func(t *testing.T) {
			err := &apierr.APIError{Status: st}
			if apierr.IsRetryable(err) {
				t.Fatalf("IsRetryable(%d) = true, want false", st)
			}
		})
	}
}

func TestIsRateLimited(t *testing.T) {
	err429 := &apierr.APIError{Status: http.StatusTooManyRequests, Message: "rate limited"}
	if !apierr.IsRateLimited(err429) {
		t.Fatalf("IsRateLimited(429) = false, want true")
	}
	if !apierr.IsRateLimited(fmt.Errorf("wrap: %w", err429)) {
		t.Fatalf("IsRateLimited(wrapped 429) = false, want true")
	}

	other := &apierr.APIError{Status: http.StatusServiceUnavailable}
	if apierr.IsRateLimited(other) {
		t.Fatalf("IsRateLimited(503) = true, want false")
	}
}

func TestIsRetryable_RealNetTimeoutSatisfies(t *testing.T) {
	// net.Dialer with tiny timeout triggers a Timeout() error on no-route addr.
	d := net.Dialer{Timeout: 1 * time.Nanosecond}
	_, err := d.Dial("tcp", "203.0.113.1:81") // TEST-NET-3; should fail fast
	if err == nil {
		// If by some fluke it connects, skip; we only care that timeouts are treated retryable.
		t.Skip("unexpectedly connected; skip environment-specific test")
	}
	if !apierr.IsRetryable(err) && !apierr.IsRetryable(fmt.Errorf("wrap: %w", err)) {
		t.Fatalf("IsRetryable(net timeout-like) = false, want true")
	}
}

func TestIsRetryable_NilAndUnknownErrors(t *testing.T) {
	if apierr.IsRetryable(nil) {
		t.Fatalf("IsRetryable(nil) = true, want false")
	}
	if apierr.IsRetryable(errors.New("some build error")) {
		t.Fatalf("IsRetryable(plain error) = true, want false")
	}
}
