package apierr_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/bodrovis/lokex/v2/internal/apierr"
)

var _ error = (*apierr.APIError)(nil)

func TestAPIError_Error_PrefersMessage(t *testing.T) {
	t.Parallel()

	e := &apierr.APIError{
		Status:  http.StatusBadRequest,
		Message: "bad payload: missing name",
	}

	if got, want := e.Error(), "bad payload: missing name"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}

func TestAPIError_Error_FallsBackToStatusText(t *testing.T) {
	t.Parallel()

	e := &apierr.APIError{
		Status: http.StatusNotFound,
	}

	want := http.StatusText(http.StatusNotFound)
	if got := e.Error(); got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}

func TestAPIError_Error_EmptyStatusAndMessage(t *testing.T) {
	t.Parallel()

	e := &apierr.APIError{}

	if got := e.Error(); got != "" {
		t.Fatalf("Error() = %q, want empty string", got)
	}
}

func TestAPIError_WrappingAndErrorsAs(t *testing.T) {
	t.Parallel()

	orig := &apierr.APIError{
		Status:  http.StatusTooManyRequests,
		Code:    1234,
		Message: "rate limited",
	}

	wrapped := fmt.Errorf("fetch bundle: %w", orig)

	var target *apierr.APIError
	if !errors.As(wrapped, &target) {
		t.Fatal("errors.As failed to find *APIError in wrapped error")
	}

	if target.Status != http.StatusTooManyRequests ||
		target.Code != 1234 ||
		target.Message != "rate limited" {
		t.Fatalf("unexpected *APIError contents: %#v", target)
	}
}

func TestAPIError_FieldsRoundtrip(t *testing.T) {
	t.Parallel()

	e := &apierr.APIError{
		Status: 500,
		Code:   500,
		Reason: "server_error",
		Details: map[string]any{
			"bucket": "global",
		},
		Raw:  `{"error":"x"}`,
		Resp: &http.Response{StatusCode: 500},
	}

	wrapped := fmt.Errorf("boom: %w", e)

	var got *apierr.APIError
	if !errors.As(wrapped, &got) {
		t.Fatal("errors.As failed")
	}

	if got.Reason != "server_error" ||
		got.Raw == "" ||
		got.Resp == nil ||
		got.Details["bucket"] != "global" {
		t.Fatalf("fields lost: %#v", got)
	}
}
