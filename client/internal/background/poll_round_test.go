package background_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/bodrovis/lokex/v2/client/internal/background"
	"github.com/bodrovis/lokex/v2/internal/apierr"
)

func TestApplyRound(t *testing.T) {
	t.Parallel()

	t.Run("context errors keep processes pending", func(t *testing.T) {
		t.Parallel()

		processMap := map[string]background.QueuedProcess{}
		pending := map[string]struct{}{
			"canceled": {},
			"deadline": {},
		}

		background.ExportApplyRound(
			processMap,
			pending,
			nil,
			map[string]error{
				"canceled": context.Canceled,
				"deadline": context.DeadlineExceeded,
			},
		)

		if len(processMap) != 0 {
			t.Fatalf("processMap = %+v, want empty", processMap)
		}

		if _, ok := pending["canceled"]; !ok {
			t.Fatal(`pending["canceled"] missing, want it to remain pending`)
		}
		if _, ok := pending["deadline"]; !ok {
			t.Fatal(`pending["deadline"] missing, want it to remain pending`)
		}
	})

	t.Run("terminal statuses update map and leave pending", func(t *testing.T) {
		t.Parallel()

		processMap := map[string]background.QueuedProcess{}
		pending := map[string]struct{}{
			"done":   {},
			"failed": {},
			"queued": {},
		}

		background.ExportApplyRound(
			processMap,
			pending,
			[]background.QueuedProcess{
				{ProcessID: "done", Status: background.StatusFinished},
				{ProcessID: "failed", Status: background.StatusFailed},
				{ProcessID: "queued", Status: background.StatusQueued},
			},
			nil,
		)

		if _, ok := pending["done"]; ok {
			t.Fatal(`pending["done"] still present`)
		}
		if _, ok := pending["failed"]; ok {
			t.Fatal(`pending["failed"] still present`)
		}
		if _, ok := pending["queued"]; !ok {
			t.Fatal(`pending["queued"] missing`)
		}

		if processMap["done"].Status != background.StatusFinished {
			t.Fatalf("done status = %q", processMap["done"].Status)
		}
	})

	t.Run("retryable error keeps process pending", func(t *testing.T) {
		t.Parallel()

		processMap := map[string]background.QueuedProcess{}
		pending := map[string]struct{}{
			"p1": {},
		}

		background.ExportApplyRound(
			processMap,
			pending,
			nil,
			map[string]error{
				"p1": &apierr.APIError{
					Status: http.StatusServiceUnavailable,
				},
			},
		)

		if _, ok := pending["p1"]; !ok {
			t.Fatal(`pending["p1"] missing, want retry`)
		}
		if len(processMap) != 0 {
			t.Fatalf("processMap = %+v, want empty", processMap)
		}
	})

	t.Run("non-retryable error preserves status", func(t *testing.T) {
		t.Parallel()

		pollErr := errors.New("bad request")

		processMap := map[string]background.QueuedProcess{
			"p1": {
				ProcessID: "p1",
				Status:    "running",
			},
		}
		pending := map[string]struct{}{
			"p1": {},
		}

		background.ExportApplyRound(
			processMap,
			pending,
			nil,
			map[string]error{
				"p1": pollErr,
			},
		)

		got := processMap["p1"]

		if got.ProcessID != "p1" {
			t.Fatalf("ProcessID = %q, want %q", got.ProcessID, "p1")
		}
		if got.Status != "running" {
			t.Fatalf("status = %q, want running", got.Status)
		}
		if !errors.Is(got.Err, pollErr) {
			t.Fatalf("Err = %v, want %v", got.Err, pollErr)
		}
		if _, ok := pending["p1"]; ok {
			t.Fatal(`pending["p1"] still present`)
		}
	})
}
