// Package apierr defines typed API errors used across lokex.
package apierr

import "net/http"

// DefaultErrCap is the maximum number of response-body bytes captured
// when constructing an APIError from a non-2xx response.
const DefaultErrCap = 8192

// APIError represents a non-2xx response from the Lokalise API or another
// HTTP service used by lokex.
//
// Callers can inspect Status, Code, Reason, and Details to decide how to
// handle the error, for example whether it is retryable.
type APIError struct {
	// Status is the HTTP status code (for example, 400, 429, or 500).
	Status int

	// Code is a service-specific numeric code, if the API returned one.
	// When absent in the payload, Code typically mirrors Status.
	Code int

	// Message is a human-readable error summary from the server.
	Message string

	// Reason is an optional machine-friendly identifier returned by the server.
	Reason string

	// Details contains arbitrary structured data returned by the API.
	Details map[string]any

	// Raw is the trimmed captured response body. It may be truncated when the
	// response exceeds DefaultErrCap.
	Raw string

	// Resp is the original HTTP response for access to status and headers.
	// Its Body must not be read by callers.
	Resp *http.Response
}

// Error implements the error interface.
// It prefers the server-provided message and falls back to the canonical
// HTTP status text.
func (e *APIError) Error() string {
	if e.Message != "" {
		return e.Message
	}

	return http.StatusText(e.Status)
}
