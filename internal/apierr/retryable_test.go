package apierr_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"syscall"
	"testing"
	"time"

	"github.com/bodrovis/lokex/internal/apierr"
)

// mock net.Error for deterministic Timeout() behaviors
type mockNetErr struct {
	msg     string
	timeout bool
}

func (m mockNetErr) Error() string { return m.msg }
func (m mockNetErr) Timeout() bool { return m.timeout }

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
		http.StatusRequestTimeout,      // 408
		http.StatusTooEarly,            // 425
		http.StatusTooManyRequests,     // 429
		http.StatusInternalServerError, // 500
		http.StatusBadGateway,          // 502
		http.StatusServiceUnavailable,  // 503
		http.StatusGatewayTimeout,      // 504
	}
	for _, st := range retryables {
		t.Run(fmt.Sprintf("status_%d_retryable", st), func(t *testing.T) {
			err := &apierr.APIError{Status: st, Message: "boom", Code: st}
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
		http.StatusBadRequest,          // 400
		http.StatusUnauthorized,        // 401
		http.StatusForbidden,           // 403
		http.StatusNotFound,            // 404
		http.StatusUnprocessableEntity, // 422
		418,                            // I'm a teapot :)
	}
	for _, st := range nonRetryables {
		t.Run(fmt.Sprintf("status_%d_nonretryable", st), func(t *testing.T) {
			err := &apierr.APIError{Status: st, Code: st}
			if apierr.IsRetryable(err) {
				t.Fatalf("IsRetryable(%d) = true, want false", st)
			}
		})
	}
}

func TestIsRetryable_FlakyIO(t *testing.T) {
	errs := []error{
		io.ErrUnexpectedEOF,
		io.EOF,
		io.ErrClosedPipe,
		syscall.ECONNRESET,
		syscall.ECONNABORTED,
		// NOTE: syscall.EPIPE is unix-only; skip to keep this test portable.
	}
	for _, e := range errs {
		if !apierr.IsRetryable(e) {
			t.Fatalf("expected retryable for %v", e)
		}
	}
}

func TestIsRetryable_ContextErrorsAreNotRetryable(t *testing.T) {
	if apierr.IsRetryable(context.Canceled) {
		t.Fatalf("context.Canceled should not be retryable")
	}
	if apierr.IsRetryable(context.DeadlineExceeded) {
		t.Fatalf("context.DeadlineExceeded should not be retryable")
	}
}

func TestIsRetryable_RealNetTimeoutSatisfies(t *testing.T) {
	// Attempt a dial that should time out immediately; envs vary, so allow skip.
	d := net.Dialer{Timeout: 1 * time.Nanosecond}
	_, err := d.Dial("tcp", "203.0.113.1:81") // TEST-NET-3 (RFC 5737)
	if err == nil {
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

func TestJitteredBackoff_BoundsAndDefault(t *testing.T) {
	// default when base <= 0
	if d := apierr.JitteredBackoff(0); d <= 0 {
		t.Fatalf("expected positive default, got %v", d)
	}

	base := 200 * time.Millisecond
	min := base / 2
	max := time.Duration(float64(base) * 1.5)

	for i := range 200 {
		got := apierr.JitteredBackoff(base)
		if got < min || got >= max {
			t.Fatalf("backoff %v out of range [%v, %v) (iteration %d)", got, min, max, i)
		}
	}
}
