package apierr

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// slurp is already size-limited; status is the HTTP status.
func Parse(slurp []byte, status int) *APIError {
	trimmed := strings.TrimSpace(string(slurp))

	// Non-JSON fallback.
	if len(trimmed) == 0 || (trimmed[0] != '{' && trimmed[0] != '[') {
		return &APIError{
			Status:  status,
			Message: http.StatusText(status),
			Reason:  "non-json error body",
			Raw:     trimmed,
		}
	}

	// Decode with UseNumber so numbers aren't forced to float64.
	dec := json.NewDecoder(strings.NewReader(trimmed))
	dec.UseNumber()
	var anyJSON any
	if err := json.Unmarshal(slurp, &anyJSON); err != nil {
		return &APIError{
			Status:  status,
			Message: http.StatusText(status),
			Reason:  "invalid json in error body",
			Raw:     trimmed,
			Details: map[string]any{"unmarshal_error": err.Error()},
		}
	}

	obj, _ := anyJSON.(map[string]any)

	// 1) Top-level: { message: string, statusCode: number, error: string }
	if msg, ok := getString(obj, "message"); ok {
		if sc, ok := getNumberAsInt(obj, "statusCode"); ok {
			if reason, ok := getString(obj, "error"); ok {
				return &APIError{
					Status:  status,
					Code:    sc,
					Message: msg,
					Reason:  reason,
					Raw:     trimmed,
					Details: obj,
				}
			}
		}
	}

	// 2) Nested: { error: { message, code, details } }
	if errAny, ok := obj["error"].(map[string]any); ok {
		msg, _ := getString(errAny, "message")
		code, okCode := getNumberAsInt(errAny, "code")
		if !okCode {
			code = status
		}

		var details map[string]any
		if detObj, ok := errAny["details"].(map[string]any); ok {
			details = detObj // pass-through
		} else if det, ok := errAny["details"]; ok {
			details = map[string]any{"details": det} // keep non-object
		} else {
			details = map[string]any{"reason": "server error without details"}
		}

		return &APIError{
			Status:  status,
			Code:    code,
			Message: coalesce(msg, http.StatusText(status)),
			Raw:     trimmed,
			Details: details,
		}
	}

	// 3) Alt top-level: { message: string, code?: number|string, errorCode?: number|string, details?: any }
	if msg, ok := getString(obj, "message"); ok {
		if code, ok := getNumberAsInt(obj, "code"); ok {
			return &APIError{
				Status:  status,
				Code:    code,
				Message: msg,
				Raw:     trimmed,
				Details: pickDetails(obj),
			}
		}
		if code, ok := getNumberAsInt(obj, "errorCode"); ok {
			return &APIError{
				Status:  status,
				Code:    code,
				Message: msg,
				Raw:     trimmed,
				Details: pickDetails(obj),
			}
		}
	}

	// 4) Fallback with reason if "error" is a string.
	reason, _ := getString(obj, "error")
	return &APIError{
		Status:  status,
		Message: coalesce(getStringOr(obj, "message", ""), http.StatusText(status)),
		Reason:  coalesce(reason, "unhandled error format"),
		Raw:     trimmed,
		Details: obj,
	}
}

func pickDetails(obj map[string]any) map[string]any {
	if detObj, ok := obj["details"].(map[string]any); ok {
		return detObj
	}
	if det, ok := obj["details"]; ok {
		return map[string]any{"details": det}
	}
	return map[string]any{"reason": "server error without details"}
}

func coalesce(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

func getString(m map[string]any, key string) (string, bool) {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s, true
		}
	}
	return "", false
}

func getStringOr(m map[string]any, key, def string) string {
	if s, ok := getString(m, key); ok {
		return s
	}
	return def
}

func getNumberAsInt(m map[string]any, key string) (int, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case json.Number:
		if i, err := strconv.Atoi(n.String()); err == nil {
			return i, true
		}
	case float64:
		return int(n), true
	case int:
		return n, true
	case string:
		// accept numeric strings: "429", "500"
		if i, err := strconv.Atoi(strings.TrimSpace(n)); err == nil {
			return i, true
		}
	}
	return 0, false
}
