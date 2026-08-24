package utils

import (
	"bytes"
	json "encoding/json/v2"
	"fmt"
)

// EncodeJSONBody JSON-encodes body into a bytes.Reader suitable for HTTP requests.
//
// Notes:
//   - JSON strings use minimal escaping, so HTML characters such as "<" are
//     not escaped unnecessarily.
//   - On marshal errors (e.g. unsupported values), returns a wrapped error.
func EncodeJSONBody(body any) (*bytes.Reader, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode body: %w", err)
	}

	return bytes.NewReader(data), nil
}
