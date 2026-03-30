// Package apierr: parsing helpers for Lokalise-style error payloads.
//
// Lokalise (and some proxies) may return different JSON error shapes.
// Parse() tries several known patterns and falls back to a generic form,
// populating APIError with best-effort fields for callers to inspect.
package apierr

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Parse converts an HTTP error body (already size-limited by caller) and
// the HTTP status code into a structured *APIError.
//   - slurp: raw response body bytes (may be empty or non-JSON)
//   - status: HTTP status code from the response
//
// Supported shapes (examples):
//  1. Top-level fields:
//     {"message":"msg","statusCode":429,"error":"Too Many Requests"}
//  2. Nested error:
//     {"error":{"message":"msg","code":429,"details":{"bucket":"global"}}}
//  3. Alternate top-level with code/errorCode (number or string):
//     {"message":"msg","code":"429","details":{...}}
//  4. Fallback: preserve "message" and "error" (string) if present; stash all fields in Details.
//
// Non-JSON bodies produce an APIError with Reason "non-json error body" and Raw=trimmed body.
func Parse(slurp []byte, status int) *APIError {
	trimmed := strings.TrimSpace(string(slurp))

	if err := validateJSONLikeBody(trimmed, status); err != nil {
		return err
	}

	anyJSON, err := decodeErrorJSON(trimmed)
	if err != nil {
		return invalidJSONError(trimmed, status, err)
	}

	obj, _ := anyJSON.(map[string]any)

	if apiErr, ok := parseTopLevelMessageStatusReason(obj, trimmed, status); ok {
		return apiErr
	}
	if apiErr, ok := parseNestedErrorObject(obj, trimmed, status); ok {
		return apiErr
	}
	if apiErr, ok := parseAltTopLevelMessageCode(obj, trimmed, status); ok {
		return apiErr
	}

	return parseGenericFallback(obj, trimmed, status)
}

func validateJSONLikeBody(trimmed string, status int) *APIError {
	if len(trimmed) == 0 || (trimmed[0] != '{' && trimmed[0] != '[') {
		return &APIError{
			Status:  status,
			Message: http.StatusText(status),
			Reason:  "non-json error body",
			Raw:     trimmed,
		}
	}
	return nil
}

func decodeErrorJSON(trimmed string) (any, error) {
	dec := json.NewDecoder(strings.NewReader(trimmed))
	dec.UseNumber()

	var anyJSON any
	if err := dec.Decode(&anyJSON); err != nil {
		return nil, err
	}
	return anyJSON, nil
}

func invalidJSONError(trimmed string, status int, err error) *APIError {
	return &APIError{
		Status:  status,
		Message: http.StatusText(status),
		Reason:  "invalid json in error body",
		Raw:     trimmed,
		Details: map[string]any{"unmarshal_error": err.Error()},
	}
}
