package apierr_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/bodrovis/lokex/v2/internal/apierr"
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
	if e.Details == nil {
		t.Fatalf("Details=nil")
	}
	wantInt1(t, e.Details["x"], "Details[x]")
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
	if e.Details == nil {
		t.Fatalf("Details=nil")
	}
	wantInt1(t, e.Details["a"], "Details[a]")
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

func TestParse_AltTopLevel_Code_StringNumber(t *testing.T) {
	body := []byte(`{"message":"bad","code":"429","details":{"a":1}}`)
	e := apierr.Parse(body, http.StatusBadRequest)
	if e.Code != 429 || e.Message != "bad" {
		t.Fatalf("got code=%d msg=%q", e.Code, e.Message)
	}
}

func TestParse_TopLevel_StatusCode_String(t *testing.T) {
	body := []byte(`{"message":"oops","statusCode":"400","error":"Bad Request"}`)
	e := apierr.Parse(body, http.StatusBadRequest)
	if e.Code != 400 || e.Reason != "Bad Request" || e.Message != "oops" {
		t.Fatalf("unexpected parse: %+v", e)
	}
}

func TestParse_NestedError_DetailsArray_Preserved(t *testing.T) {
	body := []byte(`{"error":{"message":"boom","code":500,"details":["x","y"]}}`)
	e := apierr.Parse(body, 500)
	// parser wraps non-object details as {"details": <value>}
	arr, ok := e.Details["details"].([]any)
	if !ok || len(arr) != 2 || arr[0] != "x" || arr[1] != "y" {
		t.Fatalf("details not preserved: %#v", e.Details)
	}
}

func TestParse_EmptyBody(t *testing.T) {
	e := apierr.Parse([]byte(""), http.StatusBadGateway)
	if e.Reason != "non-json error body" || e.Raw != "" {
		t.Fatalf("unexpected: %+v", e)
	}
}

// helper
func wantInt1(t *testing.T, v any, field string) {
	t.Helper()
	switch x := v.(type) {
	case float64:
		if int(x) != 1 {
			t.Fatalf("%s=%v want 1 (float64)", field, v)
		}
	case json.Number:
		if x.String() != "1" {
			t.Fatalf("%s=%v want 1 (json.Number)", field, v)
		}
	case string:
		if x != "1" {
			t.Fatalf("%s=%q want \"1\" (string)", field, x)
		}
	default:
		t.Fatalf("%s has unexpected type %T (value=%v); want number-like 1", field, v, v)
	}
}
