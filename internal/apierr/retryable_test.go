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

	"github.com/bodrovis/lokex/v2/internal/apierr"
)

// mock net.Error for deterministic Timeout() behaviors
type mockNetErr struct {
	msg     string
	timeout bool
}

func (m mockNetErr) Error() string { return m.msg }
func (m mockNetErr) Timeout() bool { return m.timeout }

func TestIsRetryable_TimeoutError(t *testing.T) {
	t.Parallel()

	timeoutErr := mockNetErr{
		msg:     "i/o timeout",
		timeout: true,
	}
	nonTimeoutErr := mockNetErr{
		msg:     "conn refused",
		timeout: false,
	}

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"timeout", timeoutErr, true},
		{"wrapped timeout", fmt.Errorf("wrap: %w", timeoutErr), true},
		{"non-timeout", nonTimeoutErr, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := apierr.IsRetryable(tc.err)
			if got != tc.want {
				t.Fatalf(
					"IsRetryable(%T) = %v, want %v",
					tc.err,
					got,
					tc.want,
				)
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

func TestIsRetryable_ContextErrorsAreNotRetryable(t *testing.T) {
	if apierr.IsRetryable(context.Canceled) {
		t.Fatalf("context.Canceled should not be retryable")
	}
	if apierr.IsRetryable(context.DeadlineExceeded) {
		t.Fatalf("context.DeadlineExceeded should not be retryable")
	}
}

func TestIsRetryable_NetOpTimeoutTakesPrecedenceOverContextDeadline(t *testing.T) {
	t.Parallel()

	err := &net.OpError{
		Op:  "dial",
		Net: "tcp",
		Err: context.DeadlineExceeded,
	}

	if !apierr.IsRetryable(err) {
		t.Fatal("IsRetryable(net.OpError deadline) = false, want true")
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
	t.Parallel()

	tests := []struct {
		name string
		base time.Duration
		want time.Duration
	}{
		{
			name: "positive base",
			base: 200 * time.Millisecond,
			want: 200 * time.Millisecond,
		},
		{
			name: "zero uses default",
			base: 0,
			want: 300 * time.Millisecond,
		},
		{
			name: "negative uses default",
			base: -time.Second,
			want: 300 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			min := tt.want / 2
			max := tt.want + tt.want/2

			for i := range 200 {
				got := apierr.JitteredBackoff(tt.base)

				if got < min || got >= max {
					t.Fatalf(
						"backoff %v out of range [%v, %v) (iteration %d)",
						got,
						min,
						max,
						i,
					)
				}
			}
		})
	}
}

func TestIsRetryable_FlakyIO(t *testing.T) {
	t.Parallel()

	errs := []error{
		io.ErrUnexpectedEOF,
		io.EOF,
		io.ErrClosedPipe,
		syscall.ECONNRESET,
		syscall.EPIPE,
		syscall.ECONNABORTED,
	}

	for _, err := range errs {
		if !apierr.IsRetryable(err) {
			t.Fatalf("IsRetryable(%v) = false, want true", err)
		}
	}
}
