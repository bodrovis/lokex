package upload_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bodrovis/lokex/v2/client"
	"github.com/bodrovis/lokex/v2/client/upload"
)

// IMPORTANT:
// These tests intentionally do NOT use t.Parallel() because they patch
// package-level function vars via exported test setters.

func newTestUploader(t *testing.T) *upload.Uploader {
	t.Helper()

	cli, err := client.NewClient(token, projectID, client.WithHTTPTimeout(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	return upload.NewUploader(cli)
}

func TestUploader_UploadBatch(t *testing.T) {
	t.Run("nil uploader", func(t *testing.T) {
		var u *upload.Uploader

		got, err := u.UploadBatch(context.Background(), []upload.BatchUploadItem{
			{
				Params: upload.UploadParams{
					"filename": "test.json",
					"data":     "dGVzdA==",
				},
			},
		}, false)
		if err == nil {
			t.Fatal("UploadBatch() error = nil, want non-nil")
		}
		if err.Error() != "upload: batch: uploader/client is nil" {
			t.Fatalf("error = %q, want %q", err.Error(), "upload: batch: uploader/client is nil")
		}
		if len(got.Items) != 0 {
			t.Fatalf("got.Items len = %d, want 0 on fatal error", len(got.Items))
		}
	})

	t.Run("nil client", func(t *testing.T) {
		u := upload.ExportNewUploaderWithClientForTest(nil)

		got, err := u.UploadBatch(context.Background(), []upload.BatchUploadItem{
			{
				Params: upload.UploadParams{
					"filename": "test.json",
					"data":     "dGVzdA==",
				},
			},
		}, false)
		if err == nil {
			t.Fatal("UploadBatch() error = nil, want non-nil")
		}
		if err.Error() != "upload: batch: uploader/client is nil" {
			t.Fatalf("error = %q, want %q", err.Error(), "upload: batch: uploader/client is nil")
		}
		if len(got.Items) != 0 {
			t.Fatalf("got.Items len = %d, want 0 on fatal error", len(got.Items))
		}
	})

	t.Run("nil context uses background", func(t *testing.T) {
		restoreSingle := upload.ExportSetBatchUploadSingleForTest(
			func(_ *upload.Uploader, ctx context.Context, params upload.UploadParams, srcPath string) (string, error) {
				if ctx == nil {
					t.Fatal("ctx = nil, want non-nil")
				}
				if ctx.Err() != nil {
					t.Fatalf("ctx.Err() = %v, want nil", ctx.Err())
				}
				if srcPath != "" {
					t.Fatalf("srcPath = %q, want empty string when item SrcPath is empty", srcPath)
				}
				if got := params["filename"]; got != "test.json" {
					t.Fatalf("filename = %v, want %q", got, "test.json")
				}
				return "pid-123", nil
			},
		)
		defer restoreSingle()

		u := newTestUploader(t)

		//lint:ignore SA1012 intentionally passing nil context in this test
		got, err := u.UploadBatch(nil, []upload.BatchUploadItem{ //nolint:staticcheck // nil ctx is required for this test
			{
				Params: upload.UploadParams{
					"filename": "test.json",
					"data":     "dGVzdA==",
				},
			},
		}, false)
		if err != nil {
			t.Fatalf("UploadBatch() unexpected error = %v", err)
		}
		if len(got.Items) != 1 {
			t.Fatalf("got.Items len = %d, want 1", len(got.Items))
		}
		if got.Items[0].ProcessID != "pid-123" {
			t.Fatalf("process id = %q, want %q", got.Items[0].ProcessID, "pid-123")
		}
		if got.Items[0].Err != nil {
			t.Fatalf("item err = %v, want nil", got.Items[0].Err)
		}
		if got.Items[0].SrcPath != "test.json" {
			t.Fatalf("item SrcPath = %q, want %q", got.Items[0].SrcPath, "test.json")
		}
	})

	t.Run("empty batch", func(t *testing.T) {
		u := newTestUploader(t)

		got, err := u.UploadBatch(context.Background(), nil, false)
		if err != nil {
			t.Fatalf("UploadBatch() unexpected error = %v", err)
		}
		if got.Items == nil {
			return
		}
		if len(got.Items) != 0 {
			t.Fatalf("got.Items len = %d, want 0", len(got.Items))
		}
	})
}

func TestUploader_UploadBatch_ContextAlreadyCanceled(t *testing.T) {
	u := newTestUploader(t)

	restoreSingle := upload.ExportSetBatchUploadSingleForTest(
		func(_ *upload.Uploader, _ context.Context, _ upload.UploadParams, _ string) (string, error) {
			t.Fatal("batchUploadSingleFn was called, want no calls when ctx is already canceled")
			return "", nil
		},
	)
	defer restoreSingle()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got, err := u.UploadBatch(ctx, []upload.BatchUploadItem{
		{
			Params: upload.UploadParams{
				"filename": "a.json",
				"data":     "QQ==",
			},
			SrcPath: "a.json",
		},
	}, false)
	if err == nil {
		t.Fatal("UploadBatch() error = nil, want non-nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if len(got.Items) != 0 {
		t.Fatalf("got.Items len = %d, want 0 on fatal error", len(got.Items))
	}
}

func TestUploader_UploadBatch_ContextDeadlineExceeded(t *testing.T) {
	u := newTestUploader(t)

	restoreSingle := upload.ExportSetBatchUploadSingleForTest(
		func(_ *upload.Uploader, _ context.Context, _ upload.UploadParams, _ string) (string, error) {
			t.Fatal("batchUploadSingleFn was called, want no calls when ctx deadline already exceeded")
			return "", nil
		},
	)
	defer restoreSingle()

	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	time.Sleep(2 * time.Millisecond)

	got, err := u.UploadBatch(ctx, []upload.BatchUploadItem{
		{
			Params: upload.UploadParams{
				"filename": "a.json",
				"data":     "QQ==",
			},
			SrcPath: "a.json",
		},
	}, false)
	if err == nil {
		t.Fatal("UploadBatch() error = nil, want non-nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want context.DeadlineExceeded", err)
	}
	if len(got.Items) != 0 {
		t.Fatalf("got.Items len = %d, want 0 on fatal error", len(got.Items))
	}
}

func TestAcquireBatchUploadSlot_Success(t *testing.T) {
	sem := make(chan struct{}, 1)

	err := upload.ExportAcquireBatchUploadSlotForTest(context.Background(), sem)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}

	select {
	case <-sem:
	default:
		t.Fatal("expected semaphore slot to be acquired")
	}
}

func TestAcquireBatchUploadSlot_ContextCanceled(t *testing.T) {
	sem := make(chan struct{}, 1)
	sem <- struct{}{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := upload.ExportAcquireBatchUploadSlotForTest(ctx, sem)
	if err == nil {
		t.Fatal("err = nil, want non-nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestUploader_UploadBatch_ConcurrencyFallbackToOneWhenNonPositive(t *testing.T) {
	restoreConcurrency := upload.ExportSetBatchUploadConcurrencyForTest(0)
	defer restoreConcurrency()

	var started int32
	enterCh := make(chan string, 2)
	releaseCh := make(chan struct{})

	restoreSingle := upload.ExportSetBatchUploadSingleForTest(
		func(_ *upload.Uploader, _ context.Context, _ upload.UploadParams, srcPath string) (string, error) {
			atomic.AddInt32(&started, 1)
			enterCh <- srcPath
			<-releaseCh
			return srcPath + "-pid", nil
		},
	)
	defer restoreSingle()

	u := newTestUploader(t)

	items := []upload.BatchUploadItem{
		{
			Params:  upload.UploadParams{"filename": "a.json", "data": "QQ=="},
			SrcPath: "a.json",
		},
		{
			Params:  upload.UploadParams{"filename": "b.json", "data": "Qg=="},
			SrcPath: "b.json",
		},
	}

	done := make(chan upload.BatchUploadResult, 1)
	errCh := make(chan error, 1)

	go func() {
		got, err := u.UploadBatch(context.Background(), items, false)
		if err != nil {
			errCh <- err
			return
		}
		done <- got
	}()

	select {
	case first := <-enterCh:
		if first != "a.json" && first != "b.json" {
			t.Fatalf("first started worker = %q, want one of input paths", first)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for first worker to start")
	}

	select {
	case second := <-enterCh:
		t.Fatalf("second worker %q started before first was released; want concurrency fallback to 1", second)
	case <-time.After(150 * time.Millisecond):
		// good
	}

	releaseCh <- struct{}{}

	select {
	case second := <-enterCh:
		if second != "a.json" && second != "b.json" {
			t.Fatalf("second started worker = %q, want one of input paths", second)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for second worker after releasing first")
	}

	releaseCh <- struct{}{}

	select {
	case err := <-errCh:
		t.Fatalf("UploadBatch() unexpected error = %v", err)
	case got := <-done:
		if len(got.Items) != 2 {
			t.Fatalf("got.Items len = %d, want 2", len(got.Items))
		}
		for i, item := range got.Items {
			if item.Err != nil {
				t.Fatalf("item[%d].Err = %v, want nil", i, item.Err)
			}
			if item.ProcessID == "" {
				t.Fatalf("item[%d].ProcessID is empty, want non-empty", i)
			}
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for UploadBatch() to finish")
	}

	if got := atomic.LoadInt32(&started); got != 2 {
		t.Fatalf("started workers = %d, want 2", got)
	}
}

func TestUploader_UploadBatch_NoPoll(t *testing.T) {
	restoreSingle := upload.ExportSetBatchUploadSingleForTest(
		func(_ *upload.Uploader, _ context.Context, _ upload.UploadParams, srcPath string) (string, error) {
			switch srcPath {
			case "a.json":
				return "pid-a", nil
			case "b.json":
				return "pid-b", nil
			default:
				return "", errors.New("unexpected srcPath: " + srcPath)
			}
		},
	)
	defer restoreSingle()

	restorePoll := upload.ExportSetPollProcessesForTest(
		func(context.Context, []string, *client.Client) ([]upload.ExportQueuedProcessForTest, error) {
			t.Fatal("pollProcessesFn was called, want no poll when poll=false")
			return nil, nil
		},
	)
	defer restorePoll()

	u := newTestUploader(t)

	got, err := u.UploadBatch(context.Background(), []upload.BatchUploadItem{
		{
			Params:  upload.UploadParams{"filename": "a.json", "data": "QQ=="},
			SrcPath: "a.json",
		},
		{
			Params:  upload.UploadParams{"filename": "b.json", "data": "Qg=="},
			SrcPath: "b.json",
		},
	}, false)
	if err != nil {
		t.Fatalf("UploadBatch() unexpected error = %v", err)
	}

	if len(got.Items) != 2 {
		t.Fatalf("got.Items len = %d, want 2", len(got.Items))
	}

	if got.Items[0].ProcessID != "pid-a" || got.Items[0].Err != nil {
		t.Fatalf("item[0] = %+v, want ProcessID=pid-a and nil error", got.Items[0])
	}
	if got.Items[1].ProcessID != "pid-b" || got.Items[1].Err != nil {
		t.Fatalf("item[1] = %+v, want ProcessID=pid-b and nil error", got.Items[1])
	}
	if got.HasErrors() {
		t.Fatal("HasErrors() = true, want false")
	}

	wantIDs := []string{"pid-a", "pid-b"}
	if !reflect.DeepEqual(got.SuccessfulProcessIDs(), wantIDs) {
		t.Fatalf("SuccessfulProcessIDs() = %#v, want %#v", got.SuccessfulProcessIDs(), wantIDs)
	}
}

func TestUploader_UploadBatch_Poll_PartialSuccess(t *testing.T) {
	restoreSingle := upload.ExportSetBatchUploadSingleForTest(
		func(_ *upload.Uploader, _ context.Context, _ upload.UploadParams, srcPath string) (string, error) {
			switch srcPath {
			case "a.json":
				return "p1", nil
			case "b.json":
				return "", errors.New("kickoff failed for b")
			case "c.json":
				return "p3", nil
			default:
				return "", errors.New("unexpected srcPath: " + srcPath)
			}
		},
	)
	defer restoreSingle()

	restorePoll := upload.ExportSetPollProcessesForTest(
		func(_ context.Context, ids []string, _ *client.Client) ([]upload.ExportQueuedProcessForTest, error) {
			wantIDs := []string{"p1", "p3"}
			if !reflect.DeepEqual(ids, wantIDs) {
				t.Fatalf("poll ids = %#v, want %#v", ids, wantIDs)
			}

			return []upload.ExportQueuedProcessForTest{
				{ProcessID: "p1", Status: "finished"},
				{ProcessID: "p3", Status: "failed", Message: "bad format"},
			}, nil
		},
	)
	defer restorePoll()

	u := newTestUploader(t)

	got, err := u.UploadBatch(context.Background(), []upload.BatchUploadItem{
		{Params: upload.UploadParams{"filename": "a.json", "data": "QQ=="}, SrcPath: "a.json"},
		{Params: upload.UploadParams{"filename": "b.json", "data": "Qg=="}, SrcPath: "b.json"},
		{Params: upload.UploadParams{"filename": "c.json", "data": "Qw=="}, SrcPath: "c.json"},
	}, true)
	if err != nil {
		t.Fatalf("UploadBatch() unexpected error = %v", err)
	}

	if len(got.Items) != 3 {
		t.Fatalf("got.Items len = %d, want 3", len(got.Items))
	}

	if got.Items[0].Err != nil {
		t.Fatalf("item[0].Err = %v, want nil", got.Items[0].Err)
	}
	if got.Items[0].ProcessID != "p1" {
		t.Fatalf("item[0].ProcessID = %q, want p1", got.Items[0].ProcessID)
	}

	if got.Items[1].Err == nil || got.Items[1].Err.Error() != "kickoff failed for b" {
		t.Fatalf("item[1].Err = %v, want %q", got.Items[1].Err, "kickoff failed for b")
	}

	if got.Items[2].Err == nil {
		t.Fatal("item[2].Err = nil, want failed process error")
	}
	if !strings.Contains(got.Items[2].Err.Error(), "bad format") {
		t.Fatalf("item[2].Err = %v, want message to contain %q", got.Items[2].Err, "bad format")
	}

	if !got.HasErrors() {
		t.Fatal("HasErrors() = false, want true")
	}

	wantIDs := []string{"p1"}
	if !reflect.DeepEqual(got.SuccessfulProcessIDs(), wantIDs) {
		t.Fatalf("SuccessfulProcessIDs() = %#v, want %#v", got.SuccessfulProcessIDs(), wantIDs)
	}
}

func TestUploader_UploadBatch_Poll_PropagatesPollErrorToAllStartedItems(t *testing.T) {
	restoreSingle := upload.ExportSetBatchUploadSingleForTest(
		func(_ *upload.Uploader, _ context.Context, _ upload.UploadParams, srcPath string) (string, error) {
			switch srcPath {
			case "a.json":
				return "p1", nil
			case "b.json":
				return "p2", nil
			case "c.json":
				return "", errors.New("kickoff failed for c")
			default:
				return "", errors.New("unexpected srcPath: " + srcPath)
			}
		},
	)
	defer restoreSingle()

	restorePoll := upload.ExportSetPollProcessesForTest(
		func(_ context.Context, ids []string, _ *client.Client) ([]upload.ExportQueuedProcessForTest, error) {
			wantIDs := []string{"p1", "p2"}
			if !reflect.DeepEqual(ids, wantIDs) {
				t.Fatalf("poll ids = %#v, want %#v", ids, wantIDs)
			}
			return nil, errors.New("poll backend exploded")
		},
	)
	defer restorePoll()

	u := newTestUploader(t)

	got, err := u.UploadBatch(context.Background(), []upload.BatchUploadItem{
		{Params: upload.UploadParams{"filename": "a.json", "data": "QQ=="}, SrcPath: "a.json"},
		{Params: upload.UploadParams{"filename": "b.json", "data": "Qg=="}, SrcPath: "b.json"},
		{Params: upload.UploadParams{"filename": "c.json", "data": "Qw=="}, SrcPath: "c.json"},
	}, true)
	if err != nil {
		t.Fatalf("UploadBatch() unexpected error = %v", err)
	}

	if got.Items[0].Err == nil || !strings.Contains(got.Items[0].Err.Error(), "poll backend exploded") {
		t.Fatalf("item[0].Err = %v, want wrapped poll error", got.Items[0].Err)
	}
	if got.Items[1].Err == nil || !strings.Contains(got.Items[1].Err.Error(), "poll backend exploded") {
		t.Fatalf("item[1].Err = %v, want wrapped poll error", got.Items[1].Err)
	}
	if got.Items[2].Err == nil || got.Items[2].Err.Error() != "kickoff failed for c" {
		t.Fatalf("item[2].Err = %v, want original kickoff error", got.Items[2].Err)
	}
}

func TestUploader_UploadBatch_Poll_MissingProcessResultMarksOnlyMissingOnes(t *testing.T) {
	restoreSingle := upload.ExportSetBatchUploadSingleForTest(
		func(_ *upload.Uploader, _ context.Context, _ upload.UploadParams, srcPath string) (string, error) {
			switch srcPath {
			case "a.json":
				return "p1", nil
			case "b.json":
				return "p2", nil
			default:
				return "", errors.New("unexpected srcPath: " + srcPath)
			}
		},
	)
	defer restoreSingle()

	restorePoll := upload.ExportSetPollProcessesForTest(
		func(_ context.Context, ids []string, _ *client.Client) ([]upload.ExportQueuedProcessForTest, error) {
			wantIDs := []string{"p1", "p2"}
			if !reflect.DeepEqual(ids, wantIDs) {
				t.Fatalf("poll ids = %#v, want %#v", ids, wantIDs)
			}
			return []upload.ExportQueuedProcessForTest{
				{ProcessID: "p1", Status: "finished"},
			}, nil
		},
	)
	defer restorePoll()

	u := newTestUploader(t)

	got, err := u.UploadBatch(context.Background(), []upload.BatchUploadItem{
		{Params: upload.UploadParams{"filename": "a.json", "data": "QQ=="}, SrcPath: "a.json"},
		{Params: upload.UploadParams{"filename": "b.json", "data": "Qg=="}, SrcPath: "b.json"},
	}, true)
	if err != nil {
		t.Fatalf("UploadBatch() unexpected error = %v", err)
	}

	if got.Items[0].Err != nil {
		t.Fatalf("item[0].Err = %v, want nil", got.Items[0].Err)
	}
	if got.Items[1].Err == nil {
		t.Fatal("item[1].Err = nil, want missing process result error")
	}
	if !strings.Contains(got.Items[1].Err.Error(), `no process results returned (process_id=p2)`) {
		t.Fatalf("item[1].Err = %v, want missing process result error", got.Items[1].Err)
	}
}

func TestUploader_UploadBatch_Poll_UsesInjectedStatusHandler(t *testing.T) {
	restoreSingle := upload.ExportSetBatchUploadSingleForTest(
		func(_ *upload.Uploader, _ context.Context, _ upload.UploadParams, srcPath string) (string, error) {
			return "p1", nil
		},
	)
	defer restoreSingle()

	restorePoll := upload.ExportSetPollProcessesForTest(
		func(_ context.Context, ids []string, _ *client.Client) ([]upload.ExportQueuedProcessForTest, error) {
			if !reflect.DeepEqual(ids, []string{"p1"}) {
				t.Fatalf("poll ids = %#v, want %#v", ids, []string{"p1"})
			}
			return []upload.ExportQueuedProcessForTest{
				{ProcessID: "p1", Status: "whatever", Message: "boom"},
			}, nil
		},
	)
	defer restorePoll()

	restoreStatus := upload.ExportSetBatchHandleProcessStatusForTest(
		func(processID, status, message string) (string, error) {
			if processID != "p1" {
				t.Fatalf("processID = %q, want %q", processID, "p1")
			}
			if status != "whatever" {
				t.Fatalf("status = %q, want %q", status, "whatever")
			}
			if message != "boom" {
				t.Fatalf("message = %q, want %q", message, "boom")
			}
			return "", errors.New("custom status handler error")
		},
	)
	defer restoreStatus()

	u := newTestUploader(t)

	got, err := u.UploadBatch(context.Background(), []upload.BatchUploadItem{
		{Params: upload.UploadParams{"filename": "a.json", "data": "QQ=="}, SrcPath: "a.json"},
	}, true)
	if err != nil {
		t.Fatalf("UploadBatch() unexpected error = %v", err)
	}

	if got.Items[0].Err == nil || got.Items[0].Err.Error() != "custom status handler error" {
		t.Fatalf("item[0].Err = %v, want %q", got.Items[0].Err, "custom status handler error")
	}
}

func TestUploader_UploadBatch_RespectsConcurrencyLimit(t *testing.T) {
	restoreConcurrency := upload.ExportSetBatchUploadConcurrencyForTest(2)
	defer restoreConcurrency()

	var current int32
	var maxSeen int32

	started := make(chan string, 10)
	release := make(chan struct{})

	restoreSingle := upload.ExportSetBatchUploadSingleForTest(
		func(_ *upload.Uploader, _ context.Context, _ upload.UploadParams, srcPath string) (string, error) {
			n := atomic.AddInt32(&current, 1)
			for {
				prev := atomic.LoadInt32(&maxSeen)
				if n <= prev {
					break
				}
				if atomic.CompareAndSwapInt32(&maxSeen, prev, n) {
					break
				}
			}

			started <- srcPath
			<-release
			atomic.AddInt32(&current, -1)
			return srcPath + "-pid", nil
		},
	)
	defer restoreSingle()

	u := newTestUploader(t)

	items := []upload.BatchUploadItem{
		{Params: upload.UploadParams{"filename": "a.json", "data": "QQ=="}, SrcPath: "a.json"},
		{Params: upload.UploadParams{"filename": "b.json", "data": "Qg=="}, SrcPath: "b.json"},
		{Params: upload.UploadParams{"filename": "c.json", "data": "Qw=="}, SrcPath: "c.json"},
		{Params: upload.UploadParams{"filename": "d.json", "data": "RA=="}, SrcPath: "d.json"},
		{Params: upload.UploadParams{"filename": "e.json", "data": "RQ=="}, SrcPath: "e.json"},
	}

	done := make(chan upload.BatchUploadResult, 1)
	errCh := make(chan error, 1)

	go func() {
		got, err := u.UploadBatch(context.Background(), items, false)
		if err != nil {
			errCh <- err
			return
		}
		done <- got
	}()

	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(1 * time.Second):
			t.Fatalf("timed out waiting for started worker #%d", i+1)
		}
	}

	select {
	case third := <-started:
		t.Fatalf("third worker %q started before a slot was released; concurrency limit broken", third)
	case <-time.After(150 * time.Millisecond):
		// good: only 2 workers started so far
	}

	release <- struct{}{}

	select {
	case <-started:
		// good: third started only after releasing a slot
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for third worker after release")
	}

	for i := 0; i < len(items)-1; i++ {
		release <- struct{}{}
	}

	select {
	case err := <-errCh:
		t.Fatalf("UploadBatch() unexpected error = %v", err)
	case got := <-done:
		if len(got.Items) != len(items) {
			t.Fatalf("got.Items len = %d, want %d", len(got.Items), len(items))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for UploadBatch() to finish")
	}

	if got := atomic.LoadInt32(&maxSeen); got > 2 {
		t.Fatalf("max concurrent uploads = %d, want <= 2", got)
	}
}

func TestBatchUploadResult_Helpers(t *testing.T) {
	t.Run("HasErrors", func(t *testing.T) {
		noErr := upload.BatchUploadResult{
			Items: []upload.BatchUploadResultItem{
				{ProcessID: "p1"},
				{ProcessID: "p2"},
			},
		}
		if noErr.HasErrors() {
			t.Fatal("HasErrors() = true, want false")
		}

		withErr := upload.BatchUploadResult{
			Items: []upload.BatchUploadResultItem{
				{ProcessID: "p1"},
				{ProcessID: "p2", Err: errors.New("boom")},
			},
		}
		if !withErr.HasErrors() {
			t.Fatal("HasErrors() = false, want true")
		}
	})

	t.Run("SuccessfulProcessIDs", func(t *testing.T) {
		got := upload.BatchUploadResult{
			Items: []upload.BatchUploadResultItem{
				{ProcessID: " p1 "},
				{ProcessID: "p2", Err: errors.New("boom")},
				{ProcessID: ""},
				{ProcessID: "p3"},
			},
		}.SuccessfulProcessIDs()

		want := []string{"p1", "p3"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("SuccessfulProcessIDs() = %#v, want %#v", got, want)
		}
	})
}

// Rename the 3 exported helper names below if your actual export_test.go uses
// slightly different identifiers.

func TestNewBatchUploadResultItemForTest(t *testing.T) {
	t.Run("uses trimmed SrcPath when present", func(t *testing.T) {
		got := upload.ExportNewBatchUploadResultItemForTest(7, upload.BatchUploadItem{
			SrcPath: "  /tmp/a.json  ",
			Params:  upload.UploadParams{"filename": "ignored.json"},
		})

		if got.Index != 7 {
			t.Fatalf("Index = %d, want 7", got.Index)
		}
		if got.SrcPath != "/tmp/a.json" {
			t.Fatalf("SrcPath = %q, want %q", got.SrcPath, "/tmp/a.json")
		}
	})

	t.Run("falls back to params filename", func(t *testing.T) {
		got := upload.ExportNewBatchUploadResultItemForTest(1, upload.BatchUploadItem{
			Params: upload.UploadParams{"filename": "  fallback.json  "},
		})

		if got.Index != 1 {
			t.Fatalf("Index = %d, want 1", got.Index)
		}
		if got.SrcPath != "fallback.json" {
			t.Fatalf("SrcPath = %q, want %q", got.SrcPath, "fallback.json")
		}
	})

	t.Run("keeps empty SrcPath when filename missing", func(t *testing.T) {
		got := upload.ExportNewBatchUploadResultItemForTest(2, upload.BatchUploadItem{
			Params: upload.UploadParams{"filename": 123},
		})

		if got.Index != 2 {
			t.Fatalf("Index = %d, want 2", got.Index)
		}
		if got.SrcPath != "" {
			t.Fatalf("SrcPath = %q, want empty string", got.SrcPath)
		}
	})
}

func TestCollectBatchProcessIDsForTest(t *testing.T) {
	results := []upload.BatchUploadResultItem{
		{Index: 0, ProcessID: " p1 "},
		{Index: 1, ProcessID: ""},
		{Index: 2, ProcessID: "p2", Err: errors.New("skip me")},
		{Index: 3, ProcessID: "p1"},
		{Index: 4, ProcessID: " p3 "},
	}

	gotIDs, gotMap := upload.ExportCollectBatchProcessIDsForTest(results)

	wantIDs := []string{"p1", "p3"}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("processIDs = %#v, want %#v", gotIDs, wantIDs)
	}

	wantMap := map[string][]int{
		"p1": {0, 3},
		"p3": {4},
	}
	if !reflect.DeepEqual(gotMap, wantMap) {
		t.Fatalf("idToIndexes = %#v, want %#v", gotMap, wantMap)
	}
}

func TestMarkBatchPollErrorForTest(t *testing.T) {
	results := []upload.BatchUploadResultItem{
		{Index: 0, ProcessID: "p1"},
		{Index: 1, ProcessID: "p2"},
		{Index: 2, ProcessID: "p1"},
	}

	errBoom := errors.New("poll broke")
	upload.ExportMarkBatchPollErrorForTest(
		results,
		[]string{"p1"},
		map[string][]int{
			"p1": {0, 2},
		},
		errBoom,
	)

	if !errors.Is(results[0].Err, errBoom) {
		t.Fatalf("results[0].Err = %v, want errBoom", results[0].Err)
	}
	if results[1].Err != nil {
		t.Fatalf("results[1].Err = %v, want nil", results[1].Err)
	}
	if !errors.Is(results[2].Err, errBoom) {
		t.Fatalf("results[2].Err = %v, want errBoom", results[2].Err)
	}
}

func TestPollBatchResultsForTest_SkipsEmptyProcessIDFromPollResponse(t *testing.T) {
	restorePoll := upload.ExportSetPollProcessesForTest(
		func(_ context.Context, ids []string, _ *client.Client) ([]upload.ExportQueuedProcessForTest, error) {
			if !reflect.DeepEqual(ids, []string{"p1"}) {
				t.Fatalf("poll ids = %#v, want %#v", ids, []string{"p1"})
			}

			return []upload.ExportQueuedProcessForTest{
				{ProcessID: "", Status: "failed", Message: "should be ignored"},
				{ProcessID: "p1", Status: "finished"},
			}, nil
		},
	)
	defer restorePoll()

	u := newTestUploader(t)

	results := []upload.BatchUploadResultItem{
		{Index: 0, SrcPath: "a.json", ProcessID: "p1"},
	}

	upload.ExportPollBatchResultsForTest(u, context.Background(), results)

	if results[0].Err != nil {
		t.Fatalf("results[0].Err = %v, want nil", results[0].Err)
	}
	if results[0].ProcessID != "p1" {
		t.Fatalf("results[0].ProcessID = %q, want %q", results[0].ProcessID, "p1")
	}
}

func TestPollBatchResultsForTest_SkipsUnknownProcessIDFromPollResponse(t *testing.T) {
	restorePoll := upload.ExportSetPollProcessesForTest(
		func(_ context.Context, ids []string, _ *client.Client) ([]upload.ExportQueuedProcessForTest, error) {
			if !reflect.DeepEqual(ids, []string{"p1"}) {
				t.Fatalf("poll ids = %#v, want %#v", ids, []string{"p1"})
			}

			return []upload.ExportQueuedProcessForTest{
				{ProcessID: "unknown", Status: "failed", Message: "should be ignored"},
				{ProcessID: "p1", Status: "finished"},
			}, nil
		},
	)
	defer restorePoll()

	u := newTestUploader(t)

	results := []upload.BatchUploadResultItem{
		{Index: 0, SrcPath: "a.json", ProcessID: "p1"},
	}

	upload.ExportPollBatchResultsForTest(u, context.Background(), results)

	if results[0].Err != nil {
		t.Fatalf("results[0].Err = %v, want nil", results[0].Err)
	}
	if results[0].ProcessID != "p1" {
		t.Fatalf("results[0].ProcessID = %q, want %q", results[0].ProcessID, "p1")
	}
}

func TestPollBatchResultsForTest_NoProcessIDs_ReturnsImmediately(t *testing.T) {
	restorePoll := upload.ExportSetPollProcessesForTest(
		func(context.Context, []string, *client.Client) ([]upload.ExportQueuedProcessForTest, error) {
			t.Fatal("pollProcessesFn was called, want no calls when there are no process ids")
			return nil, nil
		},
	)
	defer restorePoll()

	u := newTestUploader(t)

	results := []upload.BatchUploadResultItem{
		{Index: 0, SrcPath: "a.json", ProcessID: "", Err: nil},
		{Index: 1, SrcPath: "b.json", ProcessID: "   ", Err: nil},
		{Index: 2, SrcPath: "c.json", ProcessID: "p3", Err: errors.New("kickoff failed")},
	}

	upload.ExportPollBatchResultsForTest(u, context.Background(), results)

	if len(results) != 3 {
		t.Fatalf("len(results) = %d, want 3", len(results))
	}

	if results[0].Index != 0 || results[0].SrcPath != "a.json" || results[0].ProcessID != "" || results[0].Err != nil {
		t.Fatalf("results[0] = %+v, want unchanged", results[0])
	}
	if results[1].Index != 1 || results[1].SrcPath != "b.json" || results[1].ProcessID != "   " || results[1].Err != nil {
		t.Fatalf("results[1] = %+v, want unchanged", results[1])
	}
	if results[2].Index != 2 || results[2].SrcPath != "c.json" || results[2].ProcessID != "p3" {
		t.Fatalf("results[2] = %+v, want unchanged non-error fields", results[2])
	}
	if results[2].Err == nil {
		t.Fatal("results[2].Err = nil, want non-nil")
	}
	if results[2].Err.Error() != "kickoff failed" {
		t.Fatalf("results[2].Err = %v, want %q", results[2].Err, "kickoff failed")
	}
}

func TestBatchUploadSingleFn_DefaultDelegatesToUploadSingle(t *testing.T) {
	var u *upload.Uploader

	got, err := upload.ExportCallBatchUploadSingleForTest(
		u,
		context.Background(),
		upload.UploadParams{
			"filename": "test.json",
			"data":     "dGVzdA==",
		},
		"",
	)
	if err == nil {
		t.Fatal("error = nil, want non-nil")
	}
	if err.Error() != "upload: uploader/client is nil" {
		t.Fatalf("error = %q, want %q", err.Error(), "upload: uploader/client is nil")
	}
	if got != "" {
		t.Fatalf("got = %q, want empty string", got)
	}
}

func TestKickoffBatchUploadItemForTest_SetsErrorWhenAcquireSlotFails(t *testing.T) {
	restoreSingle := upload.ExportSetBatchUploadSingleForTest(
		func(_ *upload.Uploader, _ context.Context, _ upload.UploadParams, _ string) (string, error) {
			t.Fatal("batchUploadSingleFn was called, want no call when acquireBatchUploadSlot fails")
			return "", nil
		},
	)
	defer restoreSingle()

	u := newTestUploader(t)

	sem := make(chan struct{}, 1)
	sem <- struct{}{} // slot occupied

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := upload.BatchUploadResultItem{
		Index:   0,
		SrcPath: "a.json",
	}

	upload.ExportKickoffBatchUploadItemForTest(
		u,
		ctx,
		sem,
		upload.BatchUploadItem{
			Params:  upload.UploadParams{"filename": "a.json", "data": "QQ=="},
			SrcPath: "a.json",
		},
		&result,
	)

	if !errors.Is(result.Err, context.Canceled) {
		t.Fatalf("result.Err = %v, want context.Canceled", result.Err)
	}
	if result.ProcessID != "" {
		t.Fatalf("result.ProcessID = %q, want empty string", result.ProcessID)
	}
	if result.Index != 0 {
		t.Fatalf("result.Index = %d, want 0", result.Index)
	}
	if result.SrcPath != "a.json" {
		t.Fatalf("result.SrcPath = %q, want %q", result.SrcPath, "a.json")
	}
}
