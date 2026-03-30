package background

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/bodrovis/lokex/v2/client"
	"github.com/bodrovis/lokex/v2/internal/apierr"
	"github.com/bodrovis/lokex/v2/internal/utils"
	"golang.org/x/sync/errgroup"
)

// pollRound performs one polling round for all currently pending IDs.
// It returns successful process statuses and per-ID errors.
// Workers never block on send because resCh is buffered to len(ids).
func pollRound(
	ctx context.Context,
	c *client.Client,
	pending map[string]struct{},
	maxConcurrent int,
) ([]QueuedProcess, map[string]error) {
	ids := make([]string, 0, len(pending))
	for id := range pending {
		ids = append(ids, id)
	}

	reqr := c.Requester()

	resCh := make(chan pollResult, len(ids))

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrent)

	for _, id := range ids {
		cur := id
		g.Go(func() error {
			path := utils.ProjectPath(c.ProjectID, fmt.Sprintf("processes/%s", cur))

			var resp processResponse
			if err := reqr.DoJSON(gctx, http.MethodGet, path, nil, &resp); err != nil {
				resCh <- pollResult{id: cur, err: err}
				return nil
			}

			resCh <- pollResult{id: cur, proc: resp.ToQueuedProcess()}
			return nil
		})
	}

	// Goroutines report per-ID errors through resCh; g.Wait() only synchronizes completion.
	_ = g.Wait()
	close(resCh)

	procs := make([]QueuedProcess, 0, len(ids))
	errs := make(map[string]error, len(ids))

	for r := range resCh {
		if r.err != nil {
			errs[r.id] = r.err
			continue
		}
		procs = append(procs, r.proc)
	}

	return procs, errs
}

// applyRound updates processMap/pending based on successful statuses and errors.
func applyRound(
	processMap map[string]QueuedProcess,
	pending map[string]struct{},
	procs []QueuedProcess,
	errs map[string]error,
) {
	// Successful statuses update the latest view and remove terminal IDs.
	for _, p := range procs {
		processMap[p.ProcessID] = p
		if p.Status == StatusFinished || p.Status == StatusFailed {
			delete(pending, p.ProcessID)
		}
	}

	// Errors: retryable stays pending; non-retryable is marked failed and removed.
	// Context cancellation/deadline errors are ignored here because the caller is stopping polling.
	for id, err := range errs {
		// defensive; PollProcesses should return ctx.Err earlier
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			// don't mark failed; caller asked to stop the whole poll
			continue
		}

		if apierr.IsRetryable(err) {
			continue
		}

		processMap[id] = QueuedProcess{ProcessID: id, Status: StatusFailed}
		delete(pending, id)
	}
}
