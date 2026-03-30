package background_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bodrovis/lokex/v2/client"
	"github.com/bodrovis/lokex/v2/client/internal/background"
)

func TestPollProcesses_EmptyInput_ReturnsEmpty(t *testing.T) {
	c := newTestClient(t)

	res, err := background.PollProcesses(context.Background(), nil, c)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(res) != 0 {
		t.Fatalf("want empty, got %#v", res)
	}
}

func TestPollProcesses_QueuedToFinished_SingleID(t *testing.T) {
	var hits int32
	token := "tok"
	project := "proj_1"
	process := "upl_123"
	ua := "lokex-test/ua"

	errCh := make(chan error, 8)
	nonBlockingSend := func(err error) {
		if err == nil {
			return
		}
		select {
		case errCh <- err:
		default:
		}
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		curHit := atomic.AddInt32(&hits, 1)

		if got := r.Header.Get("X-Api-Token"); got != token {
			nonBlockingSend(fmt.Errorf("X-Api-Token = %q, want %q", got, token))
		}
		if got := r.Header.Get("User-Agent"); got != ua {
			nonBlockingSend(fmt.Errorf("User-Agent = %q, want %q", got, ua))
		}
		if r.Method != http.MethodGet {
			nonBlockingSend(fmt.Errorf("method = %s, want GET", r.Method))
		}

		wantPath := "/projects/" + project + "/processes/" + process
		if got := r.URL.Path; got != wantPath {
			nonBlockingSend(fmt.Errorf("path = %s, want %s", got, wantPath))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)

		// first hit → queued, second → finished
		if curHit == 1 {
			_, _ = w.Write([]byte(`{"process":{"process_id":"upl_123","status":"queued","message":"","details":{}}}`))
			return
		}
		_, _ = w.Write([]byte(`{"process":{"process_id":"upl_123","status":"finished","message":"","details":{"download_url":"https://example/file.zip"}}}`))
	}))
	defer srv.Close()

	c := newTestClient(t,
		withServer(srv),
		withToken(token),
		withProjectID(project),
		withUserAgent(ua),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	res, err := background.PollProcesses(ctx, []string{process}, c)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	select {
	case e := <-errCh:
		t.Fatal(e)
	default:
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

func TestPollProcesses_ContextCancel(t *testing.T) {
	// handler sleeps longer than our context to force cancel
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"process":{"process_id":"slow","status":"queued","message":"","details":{}}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, withServer(srv))

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	if _, err := background.PollProcesses(ctx, []string{"slow"}, c); err == nil {
		t.Fatalf("expected context error, got nil")
	}
}

func TestPollProcesses_MultipleIDs_PreservesOrder(t *testing.T) {
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

	c := newTestClient(t,
		withServer(srv),
		withProjectID("p"),
	)

	got, err := background.PollProcesses(context.Background(), []string{"a", "b"}, c)
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

func TestPollProcesses_DuplicatesAndEmpty_PreservesOrder(t *testing.T) {
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

	c := newTestClient(t,
		withServer(srv),
	)

	got, err := background.PollProcesses(context.Background(), []string{"a", "", "a", "b", "a"}, c)
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

func TestPollProcesses_Duplicates_DoNotSpamRequests(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"process":{"process_id":"a","status":"finished","message":"","details":{}}}`))
	}))
	defer srv.Close()

	c := newTestClient(t,
		withServer(srv),
	)

	got, err := background.PollProcesses(context.Background(), []string{"a", "a", "a"}, c)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("len=%d got=%#v", len(got), got)
	}
	if hits != 1 {
		t.Fatalf("hits=%d want 1", hits)
	}
}

func TestPollProcesses_NonRetryableError_MarksFailedAndContinues(t *testing.T) {
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

	c := newTestClient(t,
		withServer(srv),
		withProjectID("p"),
	)

	got, err := background.PollProcesses(context.Background(), []string{"x", "y"}, c)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len=%d, want 2; got=%#v", len(got), got)
	}
	if got[0].ProcessID != "x" || got[0].Status != background.StatusFailed {
		t.Fatalf("x: got=%#v, want failed", got[0])
	}
	if got[1].ProcessID != "y" || got[1].Status != background.StatusFinished {
		t.Fatalf("y: got=%#v, want finished", got[1])
	}
}

func TestPollProcesses_PollBudgetExpires_ReturnsQueued(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"process":{"process_id":"x","status":"queued","message":"","details":{}}}`))
	}))
	defer srv.Close()

	c := newTestClient(t,
		withServer(srv),
	)

	got, err := background.PollProcesses(context.Background(), []string{"x"}, c)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Status != background.StatusQueued {
		t.Fatalf("got=%#v", got)
	}
}

func TestPollProcesses_TransientErrorIsIgnoredThenRecovered(t *testing.T) {
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

	c := newTestClient(t,
		withServer(srv),
	)

	res, err := background.PollProcesses(context.Background(), []string{"x"}, c)
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

func TestPollProcesses_NilContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"process":{"process_id":"x","status":"finished","message":"","details":{}}}`))
	}))
	defer srv.Close()

	c := newTestClient(t,
		withServer(srv),
	)

	//lint:ignore SA1012 intentionally passing nil context in this test
	got, err := background.PollProcesses(nil, []string{"x"}, c) //nolint:staticcheck // nil ctx is required for this test
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Status != background.StatusFinished {
		t.Fatalf("%#v", got)
	}
}

// === helpers ===

type testClientConfig struct {
	token     string
	projectID string
	userAgent string
	srv       *httptest.Server
}

type testClientOption func(*testClientConfig)

func withToken(tok string) testClientOption {
	return func(c *testClientConfig) {
		c.token = tok
	}
}

func withProjectID(pid string) testClientOption {
	return func(c *testClientConfig) {
		c.projectID = pid
	}
}

func withUserAgent(ua string) testClientOption {
	return func(c *testClientConfig) {
		c.userAgent = ua
	}
}

func withServer(srv *httptest.Server) testClientOption {
	return func(c *testClientConfig) {
		c.srv = srv
	}
}

func newTestClient(t *testing.T, opts ...testClientOption) *client.Client {
	t.Helper()

	cfg := testClientConfig{
		token:     "test-token",
		projectID: "test-project",
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	clientOpts := []client.Option{
		client.WithPollWait(5*time.Millisecond, 250*time.Millisecond),
	}

	if cfg.srv != nil {
		clientOpts = append(clientOpts,
			client.WithBaseURL(cfg.srv.URL+"/"),
			client.WithHTTPClient(cfg.srv.Client()),
		)
	} else {
		clientOpts = append(clientOpts,
			client.WithBaseURL("https://example.test/api2/"),
			client.WithHTTPClient(&http.Client{}),
		)
	}

	if strings.TrimSpace(cfg.userAgent) != "" {
		clientOpts = append(clientOpts, client.WithUserAgent(cfg.userAgent))
	}

	c, err := client.NewClient(cfg.token, cfg.projectID, clientOpts...)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	return c
}
