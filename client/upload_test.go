package client_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bodrovis/lokex/client"
	"github.com/jarcoal/httpmock"
)

func TestUploader_Upload_Happy_Base64FromFile(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	targetPost := fmt.Sprintf("https://api.lokalise.com/api2/projects/%s/files/upload", projectID)
	// Kickoff returns nested process id
	httpmock.RegisterResponder("POST", targetPost, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", req.Method)
		}
		if got := req.Header.Get("X-Api-Token"); got != token {
			t.Fatalf("X-Api-Token = %q, want %q", got, token)
		}
		if ct := req.Header.Get("Content-Type"); ct != "application/json" {
			t.Fatalf("Content-Type = %q, want application/json", ct)
		}

		var got map[string]any
		if err := json.NewDecoder(req.Body).Decode(&got); err != nil {
			t.Fatalf("decode req: %v", err)
		}

		fn, ok := got["filename"].(string)
		if !ok || fn == "" {
			t.Fatalf("missing/empty filename in body: %#v", got["filename"])
		}
		if got["lang_iso"] != "en" {
			t.Fatalf("lang_iso = %v, want en", got["lang_iso"])
		}
		b64, _ := got["data"].(string)
		if b64 == "" {
			t.Fatalf("data must be base64 string")
		}

		// Validate b64 matches actual file contents
		raw, err := os.ReadFile(fn)
		if err != nil {
			t.Fatalf("read temp file: %v", err)
		}
		if b64 != base64.StdEncoding.EncodeToString(raw) {
			t.Fatalf("base64 mismatch")
		}

		return httpmock.NewStringResponse(200, `{"process":{"process_id":"upl_123"}}`), nil
	})

	// Poller
	targetGet := fmt.Sprintf("https://api.lokalise.com/api2/projects/%s/processes/upl_123", projectID)
	httpmock.RegisterResponder("GET", targetGet, httpmock.NewStringResponder(200, `{
		"process":{"process_id":"upl_123","status":"finished"}
	}`))

	cli, err := client.NewClient(token, projectID, client.WithHTTPTimeout(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	u := client.NewUploader(cli)

	// Prepare temp JSON file
	dir := t.TempDir()
	fp := filepath.Join(dir, "en.json")
	if err := os.WriteFile(fp, []byte(`{"hello":"world"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	params := client.UploadParams{
		"filename": fp, // preserved
		"lang_iso": "en",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	pid, err := u.Upload(ctx, params, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pid != "upl_123" {
		t.Fatalf("process id = %q, want upl_123", pid)
	}
}

func TestUploader_Upload_NoPoll(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	targetPost := fmt.Sprintf("https://api.lokalise.com/api2/projects/%s/files/upload", projectID)
	// kickoff returns nested process id
	httpmock.RegisterResponder("POST", targetPost, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", req.Method)
		}
		if got := req.Header.Get("X-Api-Token"); got != token {
			t.Fatalf("X-Api-Token = %q, want %q", got, token)
		}

		var got map[string]any
		if err := json.NewDecoder(req.Body).Decode(&got); err != nil {
			t.Fatalf("decode req: %v", err)
		}
		if got["filename"] == "" {
			t.Fatalf("missing filename")
		}
		return httpmock.NewStringResponse(200, `{"process":{"process_id":"upl_456"}}`), nil
	})

	cli, err := client.NewClient(token, projectID, client.WithHTTPTimeout(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	u := client.NewUploader(cli)

	// temp file
	dir := t.TempDir()
	fp := filepath.Join(dir, "en.json")
	if err := os.WriteFile(fp, []byte(`{"hello":"world"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	params := client.UploadParams{
		"filename": fp,
		"lang_iso": "en",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	pid, err := u.Upload(ctx, params, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pid != "upl_456" {
		t.Fatalf("process id = %q, want upl_456", pid)
	}
}

func TestUploader_Upload_UsesExistingDataString(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	targetPost := fmt.Sprintf("https://api.lokalise.com/api2/projects/%s/files/upload", projectID)
	httpmock.RegisterResponder("POST", targetPost, func(req *http.Request) (*http.Response, error) {
		var got map[string]any
		if err := json.NewDecoder(req.Body).Decode(&got); err != nil {
			t.Fatalf("decode req: %v", err)
		}
		if got["filename"] == "" || got["lang_iso"] != "en" {
			t.Fatalf("missing required fields: %#v", got)
		}
		// Ensure we didn't overwrite provided data
		if got["data"] != "dGVzdA==" { // base64("test")
			t.Fatalf("data overridden or wrong, got %v", got["data"])
		}
		return httpmock.NewStringResponse(200, `{"process":{"process_id":"u2"}}`), nil
	})

	targetGet := fmt.Sprintf("https://api.lokalise.com/api2/projects/%s/processes/u2", projectID)
	httpmock.RegisterResponder("GET", targetGet, httpmock.NewStringResponder(200, `{
		"process":{"process_id":"u2","status":"finished"}
	}`))

	cli, _ := client.NewClient(token, projectID, nil)
	u := client.NewUploader(cli)

	// file still must exist (we stat it)
	dir := t.TempDir()
	fp := filepath.Join(dir, "payload.json")
	if err := os.WriteFile(fp, []byte(`ignored`), 0o644); err != nil {
		t.Fatal(err)
	}

	params := client.UploadParams{
		"filename": fp,
		"lang_iso": "en",
		"data":     "dGVzdA==", // "test"
	}

	if _, err := u.Upload(context.Background(), params, true); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestUploader_Upload_ConvertsDataBytesToBase64(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	targetPost := fmt.Sprintf("https://api.lokalise.com/api2/projects/%s/files/upload", projectID)
	httpmock.RegisterResponder("POST", targetPost, func(req *http.Request) (*http.Response, error) {
		var got map[string]any
		if err := json.NewDecoder(req.Body).Decode(&got); err != nil {
			t.Fatalf("decode req: %v", err)
		}
		if got["data"] != base64.StdEncoding.EncodeToString([]byte("XYZ")) {
			t.Fatalf("data not base64'd, got %v", got["data"])
		}
		return httpmock.NewStringResponse(200, `{"process":{"process_id":"u3"}}`), nil
	})
	targetGet := fmt.Sprintf("https://api.lokalise.com/api2/projects/%s/processes/u3", projectID)
	httpmock.RegisterResponder("GET", targetGet, httpmock.NewStringResponder(200, `{
		"process":{"process_id":"u3","status":"finished"}
	}`))

	cli, _ := client.NewClient(token, projectID, nil)
	u := client.NewUploader(cli)

	dir := t.TempDir()
	fp := filepath.Join(dir, "x.json")
	if err := os.WriteFile(fp, []byte(`ignored`), 0o644); err != nil {
		t.Fatal(err)
	}

	params := client.UploadParams{
		"filename": fp,
		"lang_iso": "en",
		"data":     []byte("XYZ"),
	}

	if _, err := u.Upload(context.Background(), params, true); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestUploader_Upload_MissingFilename(t *testing.T) {
	cli, _ := client.NewClient(token, projectID, nil)
	u := client.NewUploader(cli)

	_, err := u.Upload(context.Background(), client.UploadParams{
		"lang_iso": "en",
	}, true)
	if err == nil || !strings.Contains(err.Error(), "missing 'filename'") {
		t.Fatalf("want missing filename error, got %v", err)
	}
}

func TestUploader_Upload_DirectoryIsError(t *testing.T) {
	cli, _ := client.NewClient(token, projectID, nil)
	u := client.NewUploader(cli)

	dir := t.TempDir() // directory, not file
	_, err := u.Upload(context.Background(), client.UploadParams{
		"filename": dir,
		"lang_iso": "en",
	}, true)
	if err == nil || !strings.Contains(err.Error(), "is a directory") {
		t.Fatalf("want directory error, got %v", err)
	}
}

func TestUploader_Upload_TimeoutWhilePolling(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	// Kickoff ok
	targetPost := fmt.Sprintf("https://api.lokalise.com/api2/projects/%s/files/upload", projectID)
	httpmock.RegisterResponder("POST", targetPost, httpmock.NewStringResponder(200, `{"process":{"process_id":"slow"}}`))

	// Poller: always queued → never finishes
	targetGet := fmt.Sprintf("https://api.lokalise.com/api2/projects/%s/processes/slow", projectID)
	httpmock.RegisterResponder("GET", targetGet, httpmock.NewStringResponder(200, `{
		"process":{"process_id":"slow","status":"queued"}
	}`))

	cli, _ := client.NewClient(token, projectID, nil)
	u := client.NewUploader(cli)

	dir := t.TempDir()
	fp := filepath.Join(dir, "en.json")
	if err := os.WriteFile(fp, []byte(`{"x":1}`), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	_, err := u.Upload(ctx, client.UploadParams{
		"filename": fp,
		"lang_iso": "en",
	}, true)
	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want context deadline, got %v", err)
	}
}

func TestUploader_Upload_SetsAcceptHeader_AndContentType(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	targetPost := fmt.Sprintf("https://api.lokalise.com/api2/projects/%s/files/upload", projectID)
	httpmock.RegisterResponder("POST", targetPost, func(req *http.Request) (*http.Response, error) {
		if got := req.Header.Get("Accept"); got != "application/json" {
			t.Fatalf("Accept = %q, want application/json", got)
		}
		if got := req.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("Content-Type = %q, want application/json", got)
		}
		return httpmock.NewStringResponse(200, `{"process":{"process_id":"hdr1"}}`), nil
	})

	cli, _ := client.NewClient(token, projectID, nil)
	u := client.NewUploader(cli)

	dir := t.TempDir()
	fp := filepath.Join(dir, "en.json")
	_ = os.WriteFile(fp, []byte("{}"), 0o644)

	if _, err := u.Upload(context.Background(), client.UploadParams{
		"filename": fp, "lang_iso": "en",
	}, false); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestUploader_Upload_RejectsWeirdDataType(t *testing.T) {
	cli, _ := client.NewClient(token, projectID, nil)
	u := client.NewUploader(cli)

	dir := t.TempDir()
	fp := filepath.Join(dir, "file.json")
	_ = os.WriteFile(fp, []byte("{}"), 0o644)

	_, err := u.Upload(context.Background(), client.UploadParams{
		"filename": fp, "lang_iso": "en", "data": 12345, // not string/[]byte
	}, false)
	if err == nil || !strings.Contains(err.Error(), "'data' must be string or []byte") {
		t.Fatalf("want type error, got %v", err)
	}
}

func TestUploader_Upload_ReadFileError(t *testing.T) {
	cli, _ := client.NewClient(token, projectID, nil)
	u := client.NewUploader(cli)

	// file path that doesn't exist
	_, err := u.Upload(context.Background(), client.UploadParams{
		"filename": filepath.Join(t.TempDir(), "nope.json"),
		"lang_iso": "en",
	}, false)
	if err == nil || !strings.Contains(err.Error(), "stat") {
		t.Fatalf("want stat/read error, got %v", err)
	}
}

func TestUploader_Upload_EmptyProcessIDIsError(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	targetPost := fmt.Sprintf("https://api.lokalise.com/api2/projects/%s/files/upload", projectID)
	httpmock.RegisterResponder("POST", targetPost, httpmock.NewStringResponder(200, `{"process":{"process_id":""}}`))

	cli, _ := client.NewClient(token, projectID, nil)
	u := client.NewUploader(cli)

	dir := t.TempDir()
	fp := filepath.Join(dir, "en.json")
	_ = os.WriteFile(fp, []byte("{}"), 0o644)

	_, err := u.Upload(context.Background(), client.UploadParams{
		"filename": fp, "lang_iso": "en",
	}, false)
	if err == nil || !strings.Contains(err.Error(), "empty process id") {
		t.Fatalf("want empty process id error, got %v", err)
	}
}

func TestUploader_Upload_Allows204EmptyBody_ButErrorsOnMissingProcess(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	targetPost := fmt.Sprintf("https://api.lokalise.com/api2/projects/%s/files/upload", projectID)
	// doRequest treats empty body as success → resp.Process zero-value → our check should error
	httpmock.RegisterResponder("POST", targetPost, httpmock.NewStringResponder(204, ""))

	cli, _ := client.NewClient(token, projectID, nil)
	u := client.NewUploader(cli)

	dir := t.TempDir()
	fp := filepath.Join(dir, "en.json")
	_ = os.WriteFile(fp, []byte("{}"), 0o644)

	_, err := u.Upload(context.Background(), client.UploadParams{
		"filename": fp, "lang_iso": "en",
	}, false)
	if err == nil || !strings.Contains(err.Error(), "empty process id") {
		t.Fatalf("want empty process id error, got %v", err)
	}
}

func TestUploader_Upload_RetriesOn5xxThenSucceeds(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	urlPost := fmt.Sprintf("https://api.lokalise.com/api2/projects/%s/files/upload", projectID)
	attempt := 0
	httpmock.RegisterResponder("POST", urlPost, func(*http.Request) (*http.Response, error) {
		attempt++
		if attempt == 1 {
			return httpmock.NewStringResponse(503, "try later"), nil
		}
		return httpmock.NewStringResponse(200, `{"process":{"process_id":"retry_ok"}}`), nil
	})
	// poll finished
	targetGet := fmt.Sprintf("https://api.lokalise.com/api2/projects/%s/processes/retry_ok", projectID)
	httpmock.RegisterResponder("GET", targetGet, httpmock.NewStringResponder(200, `{
		"process":{"process_id":"retry_ok","status":"finished"}
	}`))

	cli, _ := client.NewClient(token, projectID, client.WithBackoff(1*time.Millisecond, 5*time.Millisecond))
	u := client.NewUploader(cli)

	dir := t.TempDir()
	fp := filepath.Join(dir, "f.json")
	_ = os.WriteFile(fp, []byte("{}"), 0o644)

	if _, err := u.Upload(context.Background(), client.UploadParams{
		"filename": fp, "lang_iso": "en",
	}, true); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	calls := httpmock.GetCallCountInfo()
	if calls["POST "+urlPost] != 2 {
		t.Fatalf("POST attempts = %d, want 2", calls["POST "+urlPost])
	}
}

func TestUploader_Upload_DoesNotRetryOn4xx(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	urlPost := fmt.Sprintf("https://api.lokalise.com/api2/projects/%s/files/upload", projectID)
	httpmock.RegisterResponder("POST", urlPost, httpmock.NewStringResponder(400, `{"error":{"message":"bad"}}`))

	cli, _ := client.NewClient(token, projectID, client.WithBackoff(1*time.Millisecond, 5*time.Millisecond))
	u := client.NewUploader(cli)

	dir := t.TempDir()
	fp := filepath.Join(dir, "f.json")
	_ = os.WriteFile(fp, []byte("{}"), 0o644)

	_, err := u.Upload(context.Background(), client.UploadParams{
		"filename": fp, "lang_iso": "en",
	}, false)
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if got := httpmock.GetCallCountInfo()["POST "+urlPost]; got != 1 {
		t.Fatalf("POST attempts = %d, want 1 (no retries on 4xx)", got)
	}
}

func TestUploader_Upload_DecodeErrorBubbles(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	urlPost := fmt.Sprintf("https://api.lokalise.com/api2/projects/%s/files/upload", projectID)
	// malformed JSON body in response
	httpmock.RegisterResponder("POST", urlPost,
		httpmock.NewStringResponder(200, `{"process":{"process_id":`),
	)
	cli, _ := client.NewClient(token, projectID, nil)
	u := client.NewUploader(cli)

	dir := t.TempDir()
	fp := filepath.Join(dir, "f.json")
	_ = os.WriteFile(fp, []byte("{}"), 0o644)

	_, err := u.Upload(context.Background(), client.UploadParams{
		"filename": fp, "lang_iso": "en",
	}, false)
	if err == nil || !strings.Contains(err.Error(), "decode response") {
		t.Fatalf("want decode response error, got %v", err)
	}
}

func TestIntegration_Upload(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in -short mode")
	}
	if token == "secret" || projectID == "123.abc" {
		t.Skip("no real Lokalise credentials; skipping integration test")
	}

	cli, err := client.NewClient(token, projectID, nil)
	if err != nil {
		t.Fatal(err)
	}
	u := client.NewUploader(cli)

	// Each test gets its own private directory that Go deletes automatically.
	dir := t.TempDir()

	// Create the file inside the temp directory.
	fp := filepath.Join(dir, "en.json")
	if err := os.WriteFile(fp, []byte(`{"hello":"lokalise"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pid, err := u.Upload(ctx, client.UploadParams{
		"filename": fp,
		"lang_iso": "en",
	}, true)
	if err != nil {
		t.Fatalf("integration upload failed: %v", err)
	}
	if pid == "" {
		t.Fatalf("expected non-empty process id")
	}
}
