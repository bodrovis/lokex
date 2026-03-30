package background

import (
	"context"
	"time"

	"github.com/bodrovis/lokex/v2/client"
	"github.com/bodrovis/lokex/v2/internal/utils"
)

const (
	// Queued process statuses
	StatusQueued   = "queued"
	StatusFinished = "finished"
	StatusFailed   = "failed"
)

type pollResult struct {
	id   string
	proc QueuedProcess
	err  error
}

// PollProcesses polls one or more Lokalise async process IDs until each reaches a
// terminal status ("finished" or "failed"), or until the overall polling budget
// (PollMaxWait) is exhausted.
//
// Ordering rules:
//   - Returns one result per NON-empty input ID
//   - Preserves the caller’s order
//   - Preserves duplicates (same ID can appear multiple times in the output)
//
// Error handling rules:
//   - Transient request errors do NOT abort polling; that ID stays pending and
//     will be retried in the next round.
//   - Non-retryable errors for an ID mark ONLY that process as "failed" and
//     remove it from pending; polling continues for other IDs.
//   - Context cancellation / deadline aborts the whole poll and returns ctx error.
//
// Implementation notes:
//   - Each polling round does parallel GETs with a fixed concurrency cap.
//   - We buffer the result channel so workers never block on send.
//   - We enforce an overall polling budget via context.WithDeadline and return
//     best-effort results when that budget expires.
func PollProcesses(ctx context.Context, processIDs []string, c *client.Client) ([]QueuedProcess, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	wait, maxWait := c.PollInitialWait, c.PollMaxWait

	deadline := time.Now().Add(maxWait)

	// pollCtx enforces the polling budget (PollMaxWait). When it expires,
	// we should stop polling and return best-effort results (not an error),
	// unless the caller's ctx itself is canceled/deadline-exceeded.
	pollCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	ordered, processMap, pending := normalizeProcessIDs(processIDs)
	if len(pending) == 0 {
		return buildResults(ordered, processMap), nil
	}

	// Bound parallelism so we don't spam Lokalise or overload the client.
	const maxConcurrent = 6

	// Reuse a timer to avoid allocating time.After() on each round.
	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	defer timer.Stop()

	for len(pending) > 0 {
		// Caller cancellation/deadline is a hard stop (real error).
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		// Poll budget expired: stop polling and return what we have.
		if pollCtx.Err() != nil {
			break
		}

		// One round: fetch all pending statuses concurrently (bounded).
		procs, errs := pollRound(pollCtx, c, pending, maxConcurrent)

		// If caller ctx died during the round, surface that (real error).
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		// Apply outcomes to processMap/pending (single goroutine mutates maps => no locks).
		applyRound(processMap, pending, procs, errs)

		if len(pending) == 0 {
			break
		}

		// If budget expired during the round, stop now (best-effort return).
		if pollCtx.Err() != nil {
			break
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}

		sleep := min(wait, remaining)
		if sleep <= 0 {
			sleep = 10 * time.Millisecond
		}

		if err := utils.SleepWithTimer(pollCtx, timer, sleep); err != nil {
			// If caller ctx is canceled/deadline-exceeded -> error.
			if cerr := ctx.Err(); cerr != nil {
				return nil, cerr
			}
			// Otherwise it's our polling budget -> best-effort return.
			break
		}

		// Exponential backoff for next round, clipped to remaining budget.
		remaining = time.Until(deadline)
		next := min(wait*2, remaining)
		if next <= 0 {
			next = 10 * time.Millisecond
		}
		wait = next
	}

	return buildResults(ordered, processMap), nil
}
