package transport_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/bodrovis/lokex/v2/client/internal/transport"
	"github.com/bodrovis/lokex/v2/internal/apierr"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type closeTrackingReader struct {
	r      io.Reader
	closed *bool
}

func (r *closeTrackingReader) Read(p []byte) (int, error) {
	return r.r.Read(p)
}

func (r *closeTrackingReader) Close() error {
	if r.closed != nil {
		*r.closed = true
	}
	return nil
}

func TestRequester_Do(t *testing.T) {
	t.Parallel()

	t.Run("new request error is returned", func(t *testing.T) {
		t.Parallel()

		r := &transport.Requester{
			BaseURL: "http://[::1",
			Token:   "tok",
		}

		err := transport.ExportDo(
			r,
			context.Background(),
			http.MethodGet,
			"/x",
			nil,
			nil,
			http.Header{},
		)
		if err == nil {
			t.Fatal("Do() error = nil, want non-nil")
		}
		if !strings.HasPrefix(err.Error(), "join url: ") {
			t.Fatalf("error = %q, want prefix %q", err.Error(), "join url: ")
		}
	})

	t.Run("nil http client", func(t *testing.T) {
		t.Parallel()

		r := &transport.Requester{
			BaseURL: "https://example.com",
			Token:   "tok",
		}

		err := transport.ExportDo(
			r,
			context.Background(),
			http.MethodGet,
			"/x",
			nil,
			nil,
			http.Header{},
		)
		if err == nil {
			t.Fatal("Do() error = nil, want non-nil")
		}
		if err.Error() != "send request: nil http client" {
			t.Fatalf("error = %q, want %q", err.Error(), "send request: nil http client")
		}
	})
}

func TestRequester_NewRequest(t *testing.T) {
	t.Parallel()

	t.Run("join url error closes body", func(t *testing.T) {
		t.Parallel()

		closed := false
		body := &closeTrackingReader{
			r:      strings.NewReader(`{"x":1}`),
			closed: &closed,
		}

		r := &transport.Requester{
			BaseURL: "http://[::1",
			Token:   "tok",
		}

		req, err := transport.ExportNewRequest(
			r,
			context.Background(),
			http.MethodPost,
			"/x",
			body,
			http.Header{},
		)
		if err == nil {
			t.Fatal("NewRequest() error = nil, want non-nil")
		}
		if !strings.HasPrefix(err.Error(), "join url: ") {
			t.Fatalf("error = %q, want prefix %q", err.Error(), "join url: ")
		}
		if req != nil {
			t.Fatal("request != nil, want nil on error")
		}
		if !closed {
			t.Fatal("body was not closed on join url error")
		}
	})

	t.Run("create request error closes body", func(t *testing.T) {
		t.Parallel()

		closed := false
		body := &closeTrackingReader{
			r:      strings.NewReader(`{"x":1}`),
			closed: &closed,
		}

		r := &transport.Requester{
			BaseURL: "https://example.com",
			Token:   "tok",
		}

		req, err := transport.ExportNewRequest(
			r,
			context.Background(),
			"GET\nBAD",
			"/x",
			body,
			http.Header{},
		)
		if err == nil {
			t.Fatal("NewRequest() error = nil, want non-nil")
		}
		if !strings.HasPrefix(err.Error(), "create request: ") {
			t.Fatalf("error = %q, want prefix %q", err.Error(), "create request: ")
		}
		if req != nil {
			t.Fatal("request != nil, want nil on error")
		}
		if !closed {
			t.Fatal("body was not closed on create request error")
		}
	})

	t.Run("nil body is fine on error path", func(t *testing.T) {
		t.Parallel()

		r := &transport.Requester{
			BaseURL: "http://[::1",
			Token:   "tok",
		}

		req, err := transport.ExportNewRequest(
			r,
			context.Background(),
			http.MethodGet,
			"/x",
			nil,
			http.Header{},
		)
		if err == nil {
			t.Fatal("NewRequest() error = nil, want non-nil")
		}
		if req != nil {
			t.Fatal("request != nil, want nil on error")
		}
	})
}

func TestMergeHeaders(t *testing.T) {
	t.Parallel()

	t.Run("empty source values are skipped", func(t *testing.T) {
		t.Parallel()

		dst := http.Header{
			"X-Existing": []string{"keep"},
		}
		src := http.Header{
			"X-Empty": []string{},
			"X-New":   []string{"value"},
		}

		transport.ExportMergeHeaders(dst, src)

		if _, ok := dst["X-Empty"]; ok {
			t.Fatal("X-Empty present in dst, want it skipped")
		}
		if got := dst.Get("X-New"); got != "value" {
			t.Fatalf("X-New = %q, want %q", got, "value")
		}
		if got := dst.Get("X-Existing"); got != "keep" {
			t.Fatalf("X-Existing = %q, want %q", got, "keep")
		}
	})
}

func TestHandleResponse(t *testing.T) {
	t.Parallel()

	t.Run("nil target returns nil", func(t *testing.T) {
		t.Parallel()

		resp := &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
		}

		if err := transport.ExportHandleResponse(resp, nil); err != nil {
			t.Fatalf("HandleResponse() unexpected error = %v", err)
		}
	})
}

func TestDecodeJSONResponse(t *testing.T) {
	t.Parallel()

	t.Run("unexpected eof with short read compared to content length", func(t *testing.T) {
		t.Parallel()

		resp := &http.Response{
			StatusCode:    200,
			ContentLength: 20,
			Body:          io.NopCloser(strings.NewReader(`{"a":`)),
		}

		var v map[string]any
		err := transport.ExportDecodeJSONResponse(resp, &v)
		if err == nil {
			t.Fatal("DecodeJSONResponse() error = nil, want non-nil")
		}
		if !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("error = %v, want wrapped %v", err, io.ErrUnexpectedEOF)
		}
		if err.Error() != "read response: unexpected EOF" {
			t.Fatalf("error = %q, want %q", err.Error(), "read response: unexpected EOF")
		}
	})

	t.Run("decode error is wrapped", func(t *testing.T) {
		t.Parallel()

		resp := &http.Response{
			StatusCode:    200,
			ContentLength: int64(len(`{"a":]`)),
			Body:          io.NopCloser(strings.NewReader(`{"a":]`)),
		}

		var v map[string]any
		err := transport.ExportDecodeJSONResponse(resp, &v)
		if err == nil {
			t.Fatal("DecodeJSONResponse() error = nil, want non-nil")
		}
		if !strings.HasPrefix(err.Error(), "decode response: ") {
			t.Fatalf("error = %q, want prefix %q", err.Error(), "decode response: ")
		}
		if errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("error = %v, want non-EOF decode error", err)
		}
	})

	t.Run("trailing data is rejected", func(t *testing.T) {
		t.Parallel()

		resp := &http.Response{
			StatusCode:    200,
			ContentLength: int64(len(`{"a":1}{"b":2}`)),
			Body:          io.NopCloser(strings.NewReader(`{"a":1}{"b":2}`)),
		}

		var v map[string]any

		err := transport.ExportDecodeJSONResponse(resp, &v)
		if err == nil {
			t.Fatal("DecodeJSONResponse() error = nil, want non-nil")
		}

		if !strings.HasPrefix(err.Error(), "decode response: ") {
			t.Fatalf("error = %q, want decode response prefix", err.Error())
		}
	})

	t.Run("invalid trailing data is wrapped", func(t *testing.T) {
		t.Parallel()

		resp := &http.Response{
			StatusCode:    200,
			ContentLength: int64(len(`{"a":1}x`)),
			Body:          io.NopCloser(strings.NewReader(`{"a":1}x`)),
		}

		var v map[string]any

		err := transport.ExportDecodeJSONResponse(resp, &v)
		if err == nil {
			t.Fatal("DecodeJSONResponse() error = nil, want non-nil")
		}

		if !strings.HasPrefix(err.Error(), "decode response: ") {
			t.Fatalf("error = %q, want decode response prefix", err.Error())
		}

		if errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("error = %v, want non-EOF decode error", err)
		}
	})

	t.Run("truncated json with complete response is decode error", func(t *testing.T) {
		t.Parallel()

		body := `{"a":`

		resp := &http.Response{
			StatusCode:    200,
			ContentLength: int64(len(body)),
			Body:          io.NopCloser(strings.NewReader(body)),
		}

		var v map[string]any

		err := transport.ExportDecodeJSONResponse(resp, &v)
		if err == nil {
			t.Fatal("DecodeJSONResponse() error = nil, want non-nil")
		}

		if err.Error() != "decode response: unexpected end of JSON input" {
			t.Fatalf(
				"error = %q, want %q",
				err.Error(),
				"decode response: unexpected end of JSON input",
			)
		}

		if errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("error = %v, want decode error rather than transport EOF", err)
		}
	})
}

func TestHandleResponse_APIError(t *testing.T) {
	t.Parallel()

	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body: io.NopCloser(strings.NewReader(
			`{"message":"bad","code":400}`,
		)),
	}

	err := transport.ExportHandleResponse(resp, nil)
	if err == nil {
		t.Fatal("HandleResponse() error = nil, want non-nil")
	}

	var apiErr *apierr.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %T, want *apierr.APIError", err)
	}

	if apiErr.Message != "bad" {
		t.Fatalf("Message = %q, want %q", apiErr.Message, "bad")
	}

	if apiErr.Code != http.StatusBadRequest {
		t.Fatalf("Code = %d, want %d", apiErr.Code, http.StatusBadRequest)
	}

	if apiErr.Resp != resp {
		t.Fatal("APIError.Resp does not reference original response")
	}
}

func TestRequester_DoJSON(t *testing.T) {
	t.Parallel()

	var gotRequest *http.Request
	responseClosed := false

	responseBody := &closeTrackingReader{
		r:      strings.NewReader(`{"value":"ok"}`),
		closed: &responseClosed,
	}

	hc := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			gotRequest = req

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       responseBody,
				Header:     make(http.Header),
				Request:    req,
			}, nil
		}),
	}

	r := transport.Requester{
		BaseURL:    "https://example.com/api2/",
		Token:      "tok123",
		UserAgent:  "lokex-test",
		HTTPClient: hc,
	}

	var got struct {
		Value string `json:"value"`
	}

	err := r.DoJSON(
		context.Background(),
		http.MethodPost,
		"projects/123/files",
		strings.NewReader(`{"foo":"bar"}`),
		&got,
	)
	if err != nil {
		t.Fatalf("DoJSON() error = %v", err)
	}

	if got.Value != "ok" {
		t.Fatalf("Value = %q, want %q", got.Value, "ok")
	}

	if gotRequest == nil {
		t.Fatal("request was not sent")
	}

	if gotRequest.Header.Get("Content-Type") != "application/json" {
		t.Fatalf(
			"Content-Type = %q, want %q",
			gotRequest.Header.Get("Content-Type"),
			"application/json",
		)
	}

	if gotRequest.Header.Get("Accept") != "application/json" {
		t.Fatalf(
			"Accept = %q, want %q",
			gotRequest.Header.Get("Accept"),
			"application/json",
		)
	}

	if gotRequest.Header.Get("X-Api-Token") != "tok123" {
		t.Fatalf(
			"X-Api-Token = %q, want %q",
			gotRequest.Header.Get("X-Api-Token"),
			"tok123",
		)
	}

	if gotRequest.Header.Get("User-Agent") != "lokex-test" {
		t.Fatalf(
			"User-Agent = %q, want %q",
			gotRequest.Header.Get("User-Agent"),
			"lokex-test",
		)
	}

	if !responseClosed {
		t.Fatal("response body was not closed")
	}
}

func TestRequester_DoJSON_HTTPClientError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("transport failed")

	hc := &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, wantErr
		}),
	}

	r := transport.Requester{
		BaseURL:    "https://example.com/api2/",
		Token:      "tok123",
		UserAgent:  "lokex-test",
		HTTPClient: hc,
	}

	err := r.DoJSON(
		context.Background(),
		http.MethodGet,
		"projects/123",
		nil,
		nil,
	)

	if err == nil {
		t.Fatal("DoJSON() error = nil, want error")
	}

	if !errors.Is(err, wantErr) {
		t.Fatalf("DoJSON() error = %v, want wrapped %v", err, wantErr)
	}

	if !strings.Contains(err.Error(), "send request") {
		t.Fatalf("DoJSON() error = %q, want send request context", err)
	}
}
