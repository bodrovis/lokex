package apierr

func pickDetails(obj map[string]any) map[string]any {
	if detObj, ok := obj["details"].(map[string]any); ok {
		return detObj
	}
	if det, ok := obj["details"]; ok {
		return map[string]any{"details": det}
	}
	return map[string]any{"reason": "server error without details"}
}

func pickNestedDetails(errObj map[string]any) map[string]any {
	if detObj, ok := errObj["details"].(map[string]any); ok {
		return detObj
	}
	if det, ok := errObj["details"]; ok {
		return map[string]any{"details": det}
	}
	return map[string]any{"reason": "server error without details"}
}
