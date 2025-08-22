package utils_test

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/bodrovis/lokex/internal/utils"
)

func TestEncodeJSONBody_DisablesHTMLEscaping_AndAddsNewline(t *testing.T) {
	in := map[string]any{
		"raw": "<script>alert('x')</script>",
		"&":   "ampersand",
	}

	buf, err := utils.EncodeJSONBody(in)
	if err != nil {
		t.Fatalf("EncodeJSONBody error: %v", err)
	}
	out := buf.String()

	// 1) No HTML escaping
	if strings.Contains(out, `\u003c`) || strings.Contains(out, `\u003e`) || strings.Contains(out, `\u0026`) {
		t.Fatalf("found escaped HTML in output: %q", out)
	}

	// 2) Ends with newline (json.Encoder.Encode behavior)
	if !strings.HasSuffix(out, "\n") {
		t.Fatalf("output must end with newline, got: %q", out)
	}

	// 3) Round-trip sanity: it should be valid JSON
	var rt map[string]any
	if err := json.Unmarshal([]byte(out), &rt); err != nil {
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
