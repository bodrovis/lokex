package background_test

import (
	"testing"

	"github.com/bodrovis/lokex/v2/client/internal/background"
)

func TestBuildResults(t *testing.T) {
	t.Parallel()

	t.Run("missing process id gets queued placeholder", func(t *testing.T) {
		t.Parallel()

		got := background.ExportBuildResults(
			[]string{"p1", "missing", "", "p1"},
			map[string]background.QueuedProcess{
				"p1": {
					ProcessID: "p1",
					Status:    background.StatusFinished,
				},
			},
		)

		if len(got) != 3 {
			t.Fatalf("len(got) = %d, want %d", len(got), 3)
		}

		if got[0].ProcessID != "p1" || got[0].Status != background.StatusFinished {
			t.Fatalf("got[0] = %+v, want process p1 with status %q", got[0], background.StatusFinished)
		}

		if got[1].ProcessID != "missing" || got[1].Status != background.StatusQueued {
			t.Fatalf("got[1] = %+v, want queued placeholder for missing id", got[1])
		}

		if got[2].ProcessID != "p1" || got[2].Status != background.StatusFinished {
			t.Fatalf("got[2] = %+v, want duplicated process p1 with status %q", got[2], background.StatusFinished)
		}
	})
}
