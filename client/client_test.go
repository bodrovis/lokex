package client_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bodrovis/lokex/client"
)

func TestNewClient_DefaultBaseURL(t *testing.T) {
	token := "tok123"
	projectID := "proj456"

	c, err := client.NewClient(token, projectID, nil)
	if err != nil {
		t.Fatalf("Cannot create client")
	}

	if c.Token != token {
		t.Fatalf("Token = %q, want %q", c.Token, token)
	}
	if c.ProjectID != projectID {
		t.Fatalf("ProjectID = %q, want %q", c.ProjectID, projectID)
	}
	if c.BaseURL != "https://api.lokalise.com/api2/" {
		t.Fatalf("BaseURL = %q, want %q", c.BaseURL, "https://api.lokalise.com/api2/")
	}
}

func TestNewClient_CustomBaseURL(t *testing.T) {
	token := "tok123"
	projectID := "proj456"
	customBase := "https://custom.lokalise.test/api2/"

	c, err := client.NewClient(token, projectID, client.WithBaseURL(
		customBase,
	))
	if err != nil {
		t.Fatalf("Cannot create client")
	}

	if c.BaseURL != customBase {
		t.Fatalf("BaseURL = %q, want %q", c.BaseURL, customBase)
	}
}

func TestNewClient_OptionValidation(t *testing.T) {
	// invalid base url
	if _, err := client.NewClient("t", "p", client.WithBaseURL(":// nope")); err == nil {
		t.Fatalf("expected error for invalid base URL")
	}
	// WithHTTPClient(nil) should error
	if _, err := client.NewClient("t", "p", client.WithHTTPClient(nil)); err == nil {
		t.Fatalf("expected error for nil http client")
	}
	// trailing slash is enforced by WithBaseURL
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	baseNoSlash := srv.URL // no trailing slash
	c, err := client.NewClient("t", "p", client.WithBaseURL(baseNoSlash))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if got := c.BaseURL[len(c.BaseURL)-1:]; got != "/" {
		t.Fatalf("expected trailing slash, got %q", c.BaseURL)
	}
}

func TestNewClient_WithUserAgentAndHTTPTimeout(t *testing.T) {
	ua := "lokex-test/1.0"
	c, err := client.NewClient("t", "p",
		client.WithUserAgent(ua),
		client.WithHTTPTimeout(1500*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	// not directly asserted here; we check headers below in PollProcesses
	if c.HTTPClient.Timeout != 1500*time.Millisecond {
		t.Fatalf("timeout = %v, want 1.5s", c.HTTPClient.Timeout)
	}
}

func TestPollProcesses_QueuedToFinished_SingleID(t *testing.T) {
	var hits int32
	token := "tok"
	project := "proj_1"
	process := "upl_123"
	ua := "lokex-test/ua"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)

		// assert headers
		if got := r.Header.Get("X-Api-Token"); got != token {
			t.Fatalf("X-Api-Token = %q, want %q", got, token)
		}
		if got := r.Header.Get("User-Agent"); got != ua {
			t.Fatalf("User-Agent = %q, want %q", got, ua)
		}
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}

		// path like: /projects/<project>/processes/<id>
		wantSuffix := "/projects/" + project + "/processes/" + process
		if got := r.URL.Path; got != wantSuffix {
			t.Fatalf("path = %s, want %s", got, wantSuffix)
		}

		// first hit → queued, second → finished
		if atomic.LoadInt32(&hits) == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"process":{"process_id":"upl_123","status":"queued","message":"","details":{}}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"process":{"process_id":"upl_123","status":"finished","message":"","details":{"download_url":"https://example/file.zip"}}}`))
	}))
	defer srv.Close()

	c, err := client.NewClient(token, project,
		client.WithBaseURL(srv.URL),
		client.WithUserAgent(ua),
		client.WithHTTPClient(srv.Client()),
		client.WithPollWait(5*time.Millisecond, 250*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	res, err := c.PollProcesses(ctx, []string{process})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(res) != 1 || res[0].ProcessID != process || res[0].Status != "finished" {
		t.Fatalf("bad result: %#v", res)
	}
	if res[0].DownloadURL == "" {
		t.Fatalf("expected download url to be set on finished process")
	}
	if atomic.LoadInt32(&hits) < 2 {
		t.Fatalf("expected at least 2 polls, got %d", hits)
	}
}

func TestPollProcesses_MultipleIDs_PreservesOrder(t *testing.T) {
	project := "p"
	token := "t"

	var aHits, bHits int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects/p/processes/a":
			atomic.AddInt32(&aHits, 1)
			status := "queued"
			if atomic.LoadInt32(&aHits) >= 2 {
				status = "finished"
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"process":{"process_id":"a","status":"` + status + `","message":"","details":{}}}`))
		case "/projects/p/processes/b":
			atomic.AddInt32(&bHits, 1)
			// b finishes immediately
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"process":{"process_id":"b","status":"finished","message":"","details":{}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c, err := client.NewClient(token, project,
		client.WithBaseURL(srv.URL),
		client.WithHTTPClient(srv.Client()),
		client.WithPollWait(5*time.Millisecond, 200*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	got, err := c.PollProcesses(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	// order must be ["a","b"]
	if len(got) != 2 || got[0].ProcessID != "a" || got[1].ProcessID != "b" {
		t.Fatalf("order mismatch: %#v", got)
	}
	if got[0].Status != "finished" || got[1].Status != "finished" {
		t.Fatalf("statuses not finished: %#v", got)
	}
}

func TestPollProcesses_TransientErrorIsIgnoredThenRecovered(t *testing.T) {
	project := "p"
	token := "t"
	var hits int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// first call → 500
		if atomic.AddInt32(&hits, 1) == 1 {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		// second call → finished
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"process":{"process_id":"x","status":"finished","message":"","details":{}}}`))
	}))
	defer srv.Close()

	c, err := client.NewClient(token, project,
		client.WithBaseURL(srv.URL),
		client.WithHTTPClient(srv.Client()),
		client.WithPollWait(5*time.Millisecond, 200*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	res, err := c.PollProcesses(context.Background(), []string{"x"})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(res) != 1 || res[0].Status != "finished" {
		t.Fatalf("bad result: %#v", res)
	}
	if hits < 2 {
		t.Fatalf("expected >=2 attempts, got %d", hits)
	}
}

func TestPollProcesses_EmptyInput_ReturnsEmpty(t *testing.T) {
	c, _ := client.NewClient("t", "p")
	res, err := c.PollProcesses(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(res) != 0 {
		t.Fatalf("want empty, got %#v", res)
	}
}

func TestPollProcesses_ContextCancel(t *testing.T) {
	project := "p"
	token := "t"

	// handler sleeps longer than our context to force cancel
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"process":{"process_id":"slow","status":"queued","message":"","details":{}}}`))
	}))
	defer srv.Close()

	c, err := client.NewClient(token, project,
		client.WithBaseURL(srv.URL),
		client.WithHTTPClient(srv.Client()),
		client.WithPollWait(10*time.Millisecond, 500*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := c.PollProcesses(ctx, []string{"slow"}); err == nil {
		t.Fatalf("expected context error, got nil")
	}
}

func TestPollProcesses_DuplicatesAndEmpty_PreservesOrder(t *testing.T) {
	project := "p"
	token := "t"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects/p/processes/a":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"process":{"process_id":"a","status":"finished","message":"","details":{}}}`))
		case "/projects/p/processes/b":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"process":{"process_id":"b","status":"finished","message":"","details":{}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c, err := client.NewClient(token, project,
		client.WithBaseURL(srv.URL),
		client.WithHTTPClient(srv.Client()),
		client.WithPollWait(5*time.Millisecond, 200*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	got, err := c.PollProcesses(context.Background(), []string{"a", "", "a", "b", "a"})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// empty IDs are skipped; duplicates preserved
	if len(got) != 4 {
		t.Fatalf("len=%d, want 4; got=%#v", len(got), got)
	}
	want := []string{"a", "a", "b", "a"}
	for i := range want {
		if got[i].ProcessID != want[i] {
			t.Fatalf("order mismatch at %d: got %q want %q; full=%#v", i, got[i].ProcessID, want[i], got)
		}
	}
}

func TestPollProcesses_NonRetryableError_MarksFailedAndContinues(t *testing.T) {
	project := "p"
	token := "t"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects/p/processes/x":
			// 404 is non-retryable -> should mark failed and stop polling x
			http.NotFound(w, r)
		case "/projects/p/processes/y":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"process":{"process_id":"y","status":"finished","message":"","details":{}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c, err := client.NewClient(token, project,
		client.WithBaseURL(srv.URL),
		client.WithHTTPClient(srv.Client()),
		client.WithPollWait(5*time.Millisecond, 200*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	got, err := c.PollProcesses(context.Background(), []string{"x", "y"})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len=%d, want 2; got=%#v", len(got), got)
	}
	if got[0].ProcessID != "x" || got[0].Status != client.StatusFailed {
		t.Fatalf("x: got=%#v, want failed", got[0])
	}
	if got[1].ProcessID != "y" || got[1].Status != client.StatusFinished {
		t.Fatalf("y: got=%#v, want finished", got[1])
	}
}
