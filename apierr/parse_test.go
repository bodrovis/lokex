package apierr_test

import (
	"net/http"
	"testing"

	"github.com/bodrovis/lokex/apierr"
)

func TestParse_NonJSON(t *testing.T) {
	body := []byte("gateway exploded lol")
	st := http.StatusBadGateway

	e := apierr.Parse(body, st)
	if e.Status != st {
		t.Fatalf("Status=%d want %d", e.Status, st)
	}
	if e.Message != http.StatusText(st) {
		t.Fatalf("Message=%q want %q", e.Message, http.StatusText(st))
	}
	if e.Reason != "non-json error body" {
		t.Fatalf("Reason=%q want %q", e.Reason, "non-json error body")
	}
	if e.Raw != "gateway exploded lol" {
		t.Fatalf("Raw=%q want %q", e.Raw, "gateway exploded lol")
	}
}

func TestParse_InvalidJSON(t *testing.T) {
	body := []byte("{oops")
	st := http.StatusInternalServerError

	e := apierr.Parse(body, st)
	if e.Status != st {
		t.Fatalf("Status=%d want %d", e.Status, st)
	}
	if e.Message != http.StatusText(st) {
		t.Fatalf("Message=%q want %q", e.Message, http.StatusText(st))
	}
	if e.Reason != "invalid json in error body" {
		t.Fatalf("Reason=%q want %q", e.Reason, "invalid json in error body")
	}
	if e.Details == nil || e.Details["unmarshal_error"] == nil {
		t.Fatalf("Details should contain unmarshal_error, got: %#v", e.Details)
	}
	if e.Raw != "{oops" {
		t.Fatalf("Raw=%q want %q", e.Raw, "{oops")
	}
}

func TestParse_TopLevelShape(t *testing.T) {
	body := []byte(`{"message":"oops","statusCode":400,"error":"Bad Request","extra":"x"}`)
	st := http.StatusBadRequest

	e := apierr.Parse(body, st)
	if e.Status != st {
		t.Fatalf("Status=%d want %d", e.Status, st)
	}
	if e.Code != 400 {
		t.Fatalf("Code=%d want 400", e.Code)
	}
	if e.Message != "oops" {
		t.Fatalf("Message=%q want %q", e.Message, "oops")
	}
	if e.Reason != "Bad Request" {
		t.Fatalf("Reason=%q want %q", e.Reason, "Bad Request")
	}
	if e.Raw == "" {
		t.Fatalf("Raw should be set")
	}
	if e.Details == nil || e.Details["extra"] != "x" {
		t.Fatalf("Details missing or wrong: %#v", e.Details)
	}
}

func TestParse_NestedError_WithDetails(t *testing.T) {
	body := []byte(`{"error":{"message":"nope","code":422,"details":{"foo":"bar"}}}`)
	st := 422

	e := apierr.Parse(body, st)
	if e.Status != st {
		t.Fatalf("Status=%d want %d", e.Status, st)
	}
	if e.Code != 422 {
		t.Fatalf("Code=%d want 422", e.Code)
	}
	if e.Message != "nope" {
		t.Fatalf("Message=%q want %q", e.Message, "nope")
	}
	if e.Details == nil || e.Details["foo"] != "bar" {
		t.Fatalf("Details=%#v want foo=bar", e.Details)
	}
}

func TestParse_NestedError_NoCode_GetsHTTPStatusAsCode(t *testing.T) {
	body := []byte(`{"error":{"message":"boom","details":{"x":1}}}`)
	st := http.StatusForbidden

	e := apierr.Parse(body, st)
	if e.Status != st {
		t.Fatalf("Status=%d want %d", e.Status, st)
	}
	if e.Code != st {
		t.Fatalf("Code=%d want %d", e.Code, st)
	}
	if e.Message != "boom" {
		t.Fatalf("Message=%q want %q", e.Message, "boom")
	}
	// details should pass through
	if e.Details == nil || e.Details["x"] != float64(1) {
		t.Fatalf("Details=%#v want x=1", e.Details)
	}
}

func TestParse_AltTopLevel_Code(t *testing.T) {
	body := []byte(`{"message":"bad","code":499,"details":{"a":1}}`)
	st := http.StatusBadRequest

	e := apierr.Parse(body, st)
	if e.Status != st {
		t.Fatalf("Status=%d want %d", e.Status, st)
	}
	if e.Code != 499 {
		t.Fatalf("Code=%d want 499", e.Code)
	}
	if e.Message != "bad" {
		t.Fatalf("Message=%q want %q", e.Message, "bad")
	}
	if e.Details == nil || e.Details["a"] != float64(1) {
		t.Fatalf("Details=%#v want a=1", e.Details)
	}
}

func TestParse_AltTopLevel_ErrorCode(t *testing.T) {
	body := []byte(`{"message":"kaput","errorCode":4711}`)
	st := http.StatusBadRequest

	e := apierr.Parse(body, st)
	if e.Status != st {
		t.Fatalf("Status=%d want %d", e.Status, st)
	}
	if e.Code != 4711 {
		t.Fatalf("Code=%d want 4711", e.Code)
	}
	if e.Message != "kaput" {
		t.Fatalf("Message=%q want %q", e.Message, "kaput")
	}
	// Details default when absent
	if e.Details == nil || e.Details["reason"] == nil {
		t.Fatalf("Details should have default reason when no details present, got %#v", e.Details)
	}
}

func TestParse_FallbackUnhandled_UsesStatusText(t *testing.T) {
	body := []byte(`{"foo":"bar"}`)
	st := 418 // I'm a teapot

	e := apierr.Parse(body, st)
	if e.Status != st {
		t.Fatalf("Status=%d want %d", e.Status, st)
	}
	if e.Message != http.StatusText(st) {
		t.Fatalf("Message=%q want %q", e.Message, http.StatusText(st))
	}
	if e.Reason != "unhandled error format" {
		t.Fatalf("Reason=%q want %q", e.Reason, "unhandled error format")
	}
	if e.Details == nil || e.Details["foo"] != "bar" {
		t.Fatalf("Details=%#v want foo=bar", e.Details)
	}
}

func TestParse_TrimsRaw(t *testing.T) {
	body := []byte("   {\"message\":\"oops\",\"code\":123}  \n")
	st := 400

	e := apierr.Parse(body, st)
	if e.Raw != `{"message":"oops","code":123}` {
		t.Fatalf("Raw=%q not trimmed as expected", e.Raw)
	}
}
