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

var (
	pollRoundFn     = pollRound
	newTimer        = time.NewTimer
	sleepWithTimer  = utils.SleepWithTimer
	nextSleepWaitFn = nextSleepWait
)

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

	wait, deadline, pollCtx, cancel := newPollContext(ctx, c)
	defer cancel()

	ordered, processMap, pending := normalizeProcessIDs(processIDs)
	if len(pending) == 0 {
		return buildResults(ordered, processMap), nil
	}

	// Bound parallelism so we don't spam Lokalise or overload the client.
	const maxConcurrent = 6

	// Reuse a timer to avoid allocating time.After() on each round.
	timer := newStoppedTimer()
	defer timer.Stop()

	for len(pending) > 0 {
		if err := callerContextErr(ctx); err != nil {
			return nil, err
		}

		// Poll budget expired: stop polling and return what we have.
		if pollBudgetExpired(pollCtx) {
			break
		}

		// One round: fetch all pending statuses concurrently (bounded).
		procs, errs := pollRoundFn(pollCtx, c, pending, maxConcurrent)

		// If caller ctx died during the round, surface that (real error).
		if err := callerContextErr(ctx); err != nil {
			return nil, err
		}

		// Apply outcomes to processMap/pending (single goroutine mutates maps => no locks).
		applyRound(processMap, pending, procs, errs)

		if len(pending) == 0 {
			break
		}

		// If budget expired during the round, stop now (best-effort return).
		if pollBudgetExpired(pollCtx) {
			break
		}

		sleep, ok := nextSleepWaitFn(wait, deadline)
		if !ok {
			break
		}

		stopped, err := sleepBetweenPollRounds(ctx, pollCtx, timer, sleep)
		if err != nil {
			return nil, err
		}
		if stopped {
			break
		}

		// Exponential backoff for next round, clipped to remaining budget.
		wait = nextPollWait(wait, deadline)
	}

	return buildResults(ordered, processMap), nil
}

func newPollContext(ctx context.Context, c *client.Client) (time.Duration, time.Time, context.Context, context.CancelFunc) {
	wait := c.PollInitialWait
	maxWait := c.PollMaxWait
	deadline := time.Now().Add(maxWait)

	// pollCtx enforces the polling budget (PollMaxWait). When it expires,
	// we should stop polling and return best-effort results (not an error),
	// unless the caller's ctx itself is canceled/deadline-exceeded.
	pollCtx, cancel := context.WithDeadline(ctx, deadline)

	return wait, deadline, pollCtx, cancel
}

func newStoppedTimer() *time.Timer {
	timer := newTimer(time.Hour)
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	return timer
}

func callerContextErr(ctx context.Context) error {
	// Caller cancellation/deadline is a hard stop (real error).
	return ctx.Err()
}

func pollBudgetExpired(pollCtx context.Context) bool {
	return pollCtx.Err() != nil
}

func nextSleepWait(wait time.Duration, deadline time.Time) (time.Duration, bool) {
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return 0, false
	}

	sleep := min(wait, remaining)
	if sleep <= 0 {
		sleep = 10 * time.Millisecond
	}

	return sleep, true
}

func sleepBetweenPollRounds(
	ctx context.Context,
	pollCtx context.Context,
	timer *time.Timer,
	sleep time.Duration,
) (bool, error) {
	if err := sleepWithTimer(pollCtx, timer, sleep); err != nil {
		// If caller ctx is canceled/deadline-exceeded -> error.
		if cerr := ctx.Err(); cerr != nil {
			return false, cerr
		}
		// Otherwise it's our polling budget -> best-effort return.
		return true, nil
	}

	return false, nil
}

func nextPollWait(wait time.Duration, deadline time.Time) time.Duration {
	remaining := time.Until(deadline)
	next := min(wait*2, remaining)
	if next <= 0 {
		next = 10 * time.Millisecond
	}
	return next
}
