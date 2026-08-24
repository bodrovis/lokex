package apierr

import "net/http"

func parseTopLevelMessageStatusReason(obj map[string]any, raw string, status int) (*APIError, bool) {
	if msg, ok := getString(obj, "message"); ok {
		if sc, ok := getNumberAsInt(obj, "statusCode"); ok {
			if reason, ok := getString(obj, "error"); ok {
				return &APIError{
					Status:  status,
					Code:    sc,
					Message: msg,
					Reason:  reason,
					Raw:     raw,
					Details: obj,
				}, true
			}
		}
	}
	return nil, false
}

func parseNestedErrorObject(obj map[string]any, raw string, status int) (*APIError, bool) {
	errObj, ok := obj["error"].(map[string]any)
	if !ok {
		return nil, false
	}

	msg, _ := getString(errObj, "message")
	code, okCode := getNumberAsInt(errObj, "code")
	if !okCode {
		code = status
	}

	return &APIError{
		Status:  status,
		Code:    code,
		Message: coalesce(msg, http.StatusText(status)),
		Raw:     raw,
		Details: pickDetails(errObj),
	}, true
}

func parseAltTopLevelMessageCode(
	obj map[string]any,
	raw string,
	status int,
) (*APIError, bool) {
	msg, ok := getString(obj, "message")
	if !ok {
		return nil, false
	}

	code, ok := getNumberAsInt(obj, "code")
	if !ok {
		code, ok = getNumberAsInt(obj, "errorCode")
	}
	if !ok {
		return nil, false
	}

	return &APIError{
		Status:  status,
		Code:    code,
		Message: msg,
		Raw:     raw,
		Details: pickDetails(obj),
	}, true
}

func parseGenericFallback(
	obj map[string]any,
	raw string,
	status int,
) *APIError {
	reason, _ := getString(obj, "error")

	return &APIError{
		Status: status,
		Code:   status,
		Message: coalesce(
			getStringOr(obj, "message", ""),
			http.StatusText(status),
		),
		Reason:  coalesce(reason, "unhandled error format"),
		Raw:     raw,
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

	return map[string]any{
		"reason": "server error without details",
	}
}
