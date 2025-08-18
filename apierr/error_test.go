package apierr_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/bodrovis/lokex/apierr"
)

// Compile-time check: APIError implements error.
var _ error = (*apierr.APIError)(nil)

func TestAPIError_Error_PrefersMessage(t *testing.T) {
	e := &apierr.APIError{
		Status:  http.StatusBadRequest,
		Message: "bad payload: missing name",
	}
	got := e.Error()
	want := "bad payload: missing name"
	if got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}

func TestAPIError_Error_FallsBackToStatusText(t *testing.T) {
	e := &apierr.APIError{
		Status: http.StatusNotFound,
		// Message empty, should fall back to status text.
	}
	got := e.Error()
	want := http.StatusText(http.StatusNotFound)
	if got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}

func TestAPIError_Error_EmptyStatusAndMessage(t *testing.T) {
	e := &apierr.APIError{} // Status=0, Message=""
	got := e.Error()
	// http.StatusText(0) returns "".
	if got != "" {
		t.Fatalf("Error() = %q, want empty string", got)
	}
}

func TestAPIError_WrappingAndErrorsAs(t *testing.T) {
	orig := &apierr.APIError{
		Status:  http.StatusTooManyRequests,
		Code:    1234,
		Message: "rate limited",
	}
	// Wrap it like client code would.
	wrapped := fmt.Errorf("fetch bundle: %w", orig)

	var target *apierr.APIError
	if !errors.As(wrapped, &target) {
		t.Fatalf("errors.As failed to find *APIError in wrapped error")
	}
	if target.Status != http.StatusTooManyRequests || target.Code != 1234 || target.Message != "rate limited" {
		t.Fatalf("unexpected *APIError contents: %#v", target)
	}
}
