package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func EncodeJSONBody(body any) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(body); err != nil {
		return nil, fmt.Errorf("encode body: %w", err)
	}
	return &buf, nil
}
