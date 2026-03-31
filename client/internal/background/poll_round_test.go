package background_test

import (
	"context"
	"testing"

	"github.com/bodrovis/lokex/v2/client/internal/background"
)

func TestApplyRound(t *testing.T) {
	t.Parallel()

	t.Run("context errors do not mark process failed and keep it pending", func(t *testing.T) {
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
}
