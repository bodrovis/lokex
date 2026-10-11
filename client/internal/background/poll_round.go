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
	resCh := make(chan pollResult, len(pending))

	var g errgroup.Group
	g.SetLimit(maxConcurrent)

	for id := range pending {
		g.Go(func() error {
			resCh <- pollProcess(ctx, c, id)
			return nil
		})
	}

	_ = g.Wait()
	close(resCh)

	return collectPollResults(resCh, len(pending))
}

func pollProcess(
	ctx context.Context,
	c *client.Client,
	id string,
) pollResult {
	reqr := c.Requester()

	path := utils.ProjectPath(
		c.ProjectID,
		fmt.Sprintf("processes/%s", id),
	)

	var resp processResponse

	if err := reqr.DoJSON(
		ctx,
		http.MethodGet,
		path,
		nil,
		&resp,
	); err != nil {
		return pollResult{id: id, err: err}
	}

	proc := resp.ToQueuedProcess()

	if proc.ProcessID != "" && proc.ProcessID != id {
		return pollResult{
			id: id,
			err: fmt.Errorf(
				"process ID mismatch: requested %q, received %q",
				id,
				proc.ProcessID,
			),
		}
	}

	proc.ProcessID = id
	return pollResult{id: id, proc: proc}
}

func collectPollResults(
	resCh <-chan pollResult,
	count int,
) ([]QueuedProcess, map[string]error) {
	procs := make([]QueuedProcess, 0, count)
	errs := make(map[string]error)

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
	for _, p := range procs {
		processMap[p.ProcessID] = p

		if p.Status == StatusFinished ||
			p.Status == StatusFailed ||
			p.Status == StatusCancelled {
			delete(pending, p.ProcessID)
		}
	}

	for id, err := range errs {
		if errors.Is(err, context.Canceled) ||
			errors.Is(err, context.DeadlineExceeded) {
			continue
		}

		if apierr.IsRetryable(err) {
			continue
		}

		p := processMap[id]
		p.ProcessID = id
		p.Err = err

		processMap[id] = p
		delete(pending, id)
	}
}
