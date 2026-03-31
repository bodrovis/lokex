package background_test

import (
	"context"
	"errors"
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

func TestPollProcesses(t *testing.T) {
	t.Run("caller context error before first round", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		cli := newTestClient(t)

		got, err := background.PollProcesses(ctx, []string{"p1"}, cli)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want %v", err, context.Canceled)
		}
		if got != nil {
			t.Fatalf("got = %+v, want nil on error", got)
		}
	})

	t.Run("poll budget expired during round stops with best effort results", func(t *testing.T) {
		restorePollRound := background.ExportSetPollRoundForTest(
			func(ctx context.Context, _ *client.Client, pending map[string]struct{}, _ int) ([]background.QueuedProcess, map[string]error) {
				<-ctx.Done()

				return []background.QueuedProcess{
					{ProcessID: "p1", Status: background.StatusQueued},
				}, nil
			},
		)
		defer restorePollRound()

		cli := newTestClient(t,
			withPollWait(time.Millisecond, 5*time.Millisecond),
		)

		got, err := background.PollProcesses(context.Background(), []string{"p1"}, cli)
		if err != nil {
			t.Fatalf("PollProcesses() unexpected error = %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("len(got) = %d, want %d", len(got), 1)
		}
		if got[0].ProcessID != "p1" || got[0].Status != background.StatusQueued {
			t.Fatalf("got[0] = %+v, want queued p1", got[0])
		}
	})

	t.Run("poll budget expired before first round returns best effort results", func(t *testing.T) {
		cli := newTestClient(t)
		cli.PollMaxWait = -time.Millisecond

		got, err := background.PollProcesses(context.Background(), []string{"p1", "", "p1"}, cli)
		if err != nil {
			t.Fatalf("PollProcesses() unexpected error = %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("len(got) = %d, want %d", len(got), 2)
		}
		if got[0].ProcessID != "p1" || got[0].Status != background.StatusQueued {
			t.Fatalf("got[0] = %+v, want queued p1", got[0])
		}
		if got[1].ProcessID != "p1" || got[1].Status != background.StatusQueued {
			t.Fatalf("got[1] = %+v, want queued duplicate p1", got[1])
		}
	})

	t.Run("next sleep wait false stops polling with best effort results", func(t *testing.T) {
		restorePollRound := background.ExportSetPollRoundForTest(
			func(_ context.Context, _ *client.Client, pending map[string]struct{}, _ int) ([]background.QueuedProcess, map[string]error) {
				return []background.QueuedProcess{
					{ProcessID: "p1", Status: background.StatusQueued},
				}, nil
			},
		)
		defer restorePollRound()

		restoreNextSleep := background.ExportSetNextSleepWaitForTest(
			func(time.Duration, time.Time) (time.Duration, bool) {
				return 0, false
			},
		)
		defer restoreNextSleep()

		cli := newTestClient(t,
			withPollWait(time.Millisecond, time.Second),
		)

		got, err := background.PollProcesses(context.Background(), []string{"p1"}, cli)
		if err != nil {
			t.Fatalf("PollProcesses() unexpected error = %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("len(got) = %d, want %d", len(got), 1)
		}
		if got[0].ProcessID != "p1" || got[0].Status != background.StatusQueued {
			t.Fatalf("got[0] = %+v, want queued p1", got[0])
		}
	})

	t.Run("sleep stop breaks polling with best effort results", func(t *testing.T) {
		restorePollRound := background.ExportSetPollRoundForTest(
			func(_ context.Context, _ *client.Client, pending map[string]struct{}, _ int) ([]background.QueuedProcess, map[string]error) {
				return []background.QueuedProcess{
					{ProcessID: "p1", Status: background.StatusQueued},
				}, nil
			},
		)
		defer restorePollRound()

		restoreSleep := background.ExportSetSleepWithTimerForTest(
			func(context.Context, *time.Timer, time.Duration) error {
				return context.DeadlineExceeded
			},
		)
		defer restoreSleep()

		cli := newTestClient(t,
			withPollWait(time.Millisecond, time.Second),
		)

		got, err := background.PollProcesses(context.Background(), []string{"p1"}, cli)
		if err != nil {
			t.Fatalf("PollProcesses() unexpected error = %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("len(got) = %d, want %d", len(got), 1)
		}
		if got[0].ProcessID != "p1" || got[0].Status != background.StatusQueued {
			t.Fatalf("got[0] = %+v, want queued p1", got[0])
		}
	})
}

func TestNewStoppedTimer(t *testing.T) {
	restore := background.ExportSetNewTimerForTest(func(time.Duration) *time.Timer {
		timer := time.NewTimer(0)
		<-timer.C
		return timer
	})
	defer restore()

	timer := background.ExportNewStoppedTimer()
	if timer == nil {
		t.Fatal("NewStoppedTimer() = nil, want non-nil")
	}
}

func TestNextSleepWait(t *testing.T) {
	t.Parallel()

	t.Run("expired deadline returns false", func(t *testing.T) {
		t.Parallel()

		got, ok := background.ExportNextSleepWait(time.Second, time.Now().Add(-time.Second))
		if ok {
			t.Fatal("ok = true, want false")
		}
		if got != 0 {
			t.Fatalf("got = %v, want %v", got, time.Duration(0))
		}
	})

	t.Run("non positive sleep falls back to ten milliseconds", func(t *testing.T) {
		t.Parallel()

		got, ok := background.ExportNextSleepWait(-time.Second, time.Now().Add(time.Second))
		if !ok {
			t.Fatal("ok = false, want true")
		}
		if got != 10*time.Millisecond {
			t.Fatalf("got = %v, want %v", got, 10*time.Millisecond)
		}
	})
}

func TestSleepBetweenPollRounds(t *testing.T) {
	restore := background.ExportSetSleepWithTimerForTest(
		func(context.Context, *time.Timer, time.Duration) error {
			return context.DeadlineExceeded
		},
	)
	defer restore()

	ctx := context.Background()
	pollCtx, cancel := context.WithCancel(context.Background())
	cancel()

	timer := time.NewTimer(time.Hour)
	defer timer.Stop()

	stopped, err := background.ExportSleepBetweenPollRounds(ctx, pollCtx, timer, time.Millisecond)
	if err != nil {
		t.Fatalf("SleepBetweenPollRounds() unexpected error = %v", err)
	}
	if !stopped {
		t.Fatal("stopped = false, want true")
	}
}

// === helpers ===

type testClientConfig struct {
	token           string
	projectID       string
	userAgent       string
	srv             *httptest.Server
	httpClient      *http.Client
	baseURL         string
	httpTimeout     time.Duration
	pollInitialWait time.Duration
	pollMaxWait     time.Duration
	maxRetries      int
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

func withPollWait(initial, max time.Duration) testClientOption {
	return func(c *testClientConfig) {
		c.pollInitialWait = initial
		c.pollMaxWait = max
	}
}

func newTestClient(t *testing.T, opts ...testClientOption) *client.Client {
	t.Helper()

	cfg := testClientConfig{
		token:           "test-token",
		projectID:       "test-project",
		httpTimeout:     2 * time.Second,
		pollInitialWait: 5 * time.Millisecond,
		pollMaxWait:     250 * time.Millisecond,
		maxRetries:      1,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	clientOpts := []client.Option{
		client.WithHTTPTimeout(cfg.httpTimeout),
		client.WithPollWait(cfg.pollInitialWait, cfg.pollMaxWait),
		client.WithMaxRetries(cfg.maxRetries),
	}

	switch {
	case strings.TrimSpace(cfg.baseURL) != "":
		clientOpts = append(clientOpts, client.WithBaseURL(cfg.baseURL))
	case cfg.srv != nil:
		clientOpts = append(clientOpts, client.WithBaseURL(cfg.srv.URL+"/"))
	default:
		clientOpts = append(clientOpts, client.WithBaseURL("https://example.test/api2/"))
	}

	switch {
	case cfg.httpClient != nil:
		clientOpts = append(clientOpts, client.WithHTTPClient(cfg.httpClient))
	case cfg.srv != nil:
		clientOpts = append(clientOpts, client.WithHTTPClient(cfg.srv.Client()))
	default:
		clientOpts = append(clientOpts, client.WithHTTPClient(&http.Client{}))
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

func TestNextPollWait_NonPositiveFallsBackToMinimum(t *testing.T) {
	got := background.ExportNextPollWait(time.Second, time.Now().Add(-time.Second))
	want := 10 * time.Millisecond

	if got != want {
		t.Fatalf("NextPollWait() = %v, want %v", got, want)
	}
}
