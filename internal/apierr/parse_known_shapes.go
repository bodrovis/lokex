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
		Details: pickNestedDetails(errObj),
	}, true
}

func parseAltTopLevelMessageCode(obj map[string]any, raw string, status int) (*APIError, bool) {
	msg, ok := getString(obj, "message")
	if !ok {
		return nil, false
	}

	if code, ok := getNumberAsInt(obj, "code"); ok {
		return &APIError{
			Status:  status,
			Code:    code,
			Message: msg,
			Raw:     raw,
			Details: pickDetails(obj),
		}, true
	}

	if code, ok := getNumberAsInt(obj, "errorCode"); ok {
		return &APIError{
			Status:  status,
			Code:    code,
			Message: msg,
			Raw:     raw,
			Details: pickDetails(obj),
		}, true
	}

	return nil, false
}

func parseGenericFallback(obj map[string]any, raw string, status int) *APIError {
	reason, _ := getString(obj, "error")
	return &APIError{
		Status:  status,
		Message: coalesce(getStringOr(obj, "message", ""), http.StatusText(status)),
		Reason:  coalesce(reason, "unhandled error format"),
		Raw:     raw,
		Details: obj,
	}
}
