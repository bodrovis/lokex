package utils_test

import (
	json "encoding/json/v2"
	"io"
	"math"
	"strings"
	"testing"

	"github.com/bodrovis/lokex/v2/internal/utils"
)

func TestEncodeJSONBody_DoesNotEscapeHTML(t *testing.T) {
	in := map[string]any{
		"raw": "<script>alert('x')</script>",
		"&":   "ampersand",
	}

	buf, err := utils.EncodeJSONBody(in)
	if err != nil {
		t.Fatalf("EncodeJSONBody error: %v", err)
	}

	outBytes, err := io.ReadAll(buf)
	if err != nil {
		t.Fatalf("read encoded json: %v", err)
	}

	out := string(outBytes)

	// HTML-specific characters should not be escaped.
	if strings.Contains(out, `\u003c`) ||
		strings.Contains(out, `\u003e`) ||
		strings.Contains(out, `\u0026`) {
		t.Fatalf("found escaped HTML in output: %q", out)
	}

	// Marshal does not append a trailing newline.
	if strings.HasSuffix(out, "\n") {
		t.Fatalf("output must not end with newline, got: %q", out)
	}

	// Round-trip sanity.
	var rt map[string]any
	if err := json.Unmarshal(outBytes, &rt); err != nil {
		t.Fatalf("round-trip unmarshal failed: %v\npayload: %q", err, out)
	}
}

func TestEncodeJSONBody_ErrorOnUnsupportedValues(t *testing.T) {
	// encoding/json rejects NaN/Inf
	in := map[string]any{
		"bad": math.Inf(1),
	}
	if _, err := utils.EncodeJSONBody(in); err == nil {
		t.Fatalf("expected error for unsupported value, got nil")
	}
}

func TestEncodeJSONBody_ErrorOnUnsupportedType(t *testing.T) {
	type payload struct {
		C chan int `json:"c"`
	}
	_, err := utils.EncodeJSONBody(payload{C: make(chan int)})
	if err == nil {
		t.Fatalf("expected error for unsupported type (chan), got nil")
	}
	// optional: assert it’s wrapped with our prefix
	if !strings.Contains(err.Error(), "encode body:") {
		t.Fatalf("error should be wrapped with context, got: %v", err)
	}
}

func TestEncodeJSONBody_NilSliceAsEmptyArray(t *testing.T) {
	in := struct {
		Items []string `json:"items"`
	}{
		Items: nil,
	}

	buf, err := utils.EncodeJSONBody(in)
	if err != nil {
		t.Fatalf("EncodeJSONBody error: %v", err)
	}

	outBytes, err := io.ReadAll(buf)
	if err != nil {
		t.Fatalf("read encoded json: %v", err)
	}

	if got, want := string(outBytes), `{"items":[]}`; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
