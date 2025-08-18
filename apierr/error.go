package apierr

import (
	"net/http"
)

type APIError struct {
	Status  int            // HTTP status
	Code    int            // service-specific code if present
	Message string         // human-ish summary
	Reason  string         // short machine-ish reason, if any
	Details map[string]any // extra fields when the server sends them
	Raw     string         // raw (trimmed) body
	Resp    *http.Response // headers etc. (body already consumed)
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return http.StatusText(e.Status)
}
