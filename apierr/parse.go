package apierr

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// parseAPIError tries multiple shapes, like your TS code.
// slurp is already size-limited; status is the HTTP status.
func Parse(slurp []byte, status int) *APIError {
	trimmed := strings.TrimSpace(string(slurp))

	// If it's not JSON, give a plain fallback.
	if len(trimmed) == 0 || (trimmed[0] != '{' && trimmed[0] != '[') {
		return &APIError{
			Status:  status,
			Message: http.StatusText(status),
			Reason:  "non-json error body",
			Raw:     trimmed,
		}
	}

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
		if det, ok := errAny["details"].(map[string]any); ok {
			details = det
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

	// 3) Alt top-level: { message: string, code?: number, errorCode?: number, details?: any }
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

	// 4) Lokalise-ish minimal: { error: { message, code } } – already covered by (2),
	//    but if they change fields, still give a sane fallback.
	return &APIError{
		Status:  status,
		Message: coalesce(getStringOr(obj, "message", ""), http.StatusText(status)),
		Reason:  "unhandled error format",
		Raw:     trimmed,
		Details: obj,
	}
}

func pickDetails(obj map[string]any) map[string]any {
	if det, ok := obj["details"].(map[string]any); ok {
		return det
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
		switch t := v.(type) {
		case string:
			return t, true
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
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n), true
		case int:
			return n, true
		case json.Number:
			if i, err := strconv.Atoi(n.String()); err == nil {
				return i, true
			}
		}
	}
	return 0, false
}
