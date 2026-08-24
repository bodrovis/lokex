package background

import (
	"context"
	"time"

	"github.com/bodrovis/lokex/v2/client"
	"github.com/bodrovis/lokex/v2/internal/utils"
)

const (
	StatusQueued   = "queued"
	StatusFinished = "finished"
	StatusFailed   = "failed"
)

type pollResult struct {
	id   string
	proc QueuedProcess
	err  error
}

var pollRoundFn = pollRound

// PollProcesses polls Lokalise async process IDs until each reaches a terminal
// status or the polling budget is exhausted.
//
// Results preserve caller order and duplicates. Empty IDs are skipped.
// Caller cancellation or deadline returns an error. Exhausting PollMaxWait
// returns the best-effort process state collected so far.
func PollProcesses(
	ctx context.Context,
	processIDs []string,
	c *client.Client,
) ([]QueuedProcess, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	wait, deadline, pollCtx, cancel := newPollContext(ctx, c)
	defer cancel()

	ordered, processMap, pending := normalizeProcessIDs(processIDs)
	if len(pending) == 0 {
		return buildResults(ordered, processMap), nil
	}

	const maxConcurrent = 6

	timer := time.NewTimer(time.Hour)
	timer.Stop()
	defer timer.Stop()

	for len(pending) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if pollBudgetExpired(deadline) {
			break
		}

		procs, errs := pollRoundFn(
			pollCtx,
			c,
			pending,
			maxConcurrent,
		)

		if err := ctx.Err(); err != nil {
			return nil, err
		}

		applyRound(processMap, pending, procs, errs)

		if len(pending) == 0 {
			break
		}

		if pollBudgetExpired(deadline) {
			break
		}

		sleep, ok := nextSleepWait(wait, deadline)
		if !ok {
			break
		}

		stopped, err := sleepBetweenPollRounds(
			ctx,
			pollCtx,
			timer,
			sleep,
		)
		if err != nil {
			return nil, err
		}
		if stopped {
			break
		}

		wait = nextPollWait(wait, deadline)
	}

	return buildResults(ordered, processMap), nil
}

func newPollContext(
	ctx context.Context,
	c *client.Client,
) (
	time.Duration,
	time.Time,
	context.Context,
	context.CancelFunc,
) {
	wait := c.PollInitialWait
	deadline := time.Now().Add(c.PollMaxWait)
	pollCtx, cancel := context.WithDeadline(ctx, deadline)

	return wait, deadline, pollCtx, cancel
}

func pollBudgetExpired(deadline time.Time) bool {
	return !time.Now().Before(deadline)
}

func nextSleepWait(
	wait time.Duration,
	deadline time.Time,
) (time.Duration, bool) {
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
	if err := utils.SleepWithTimer(pollCtx, timer, sleep); err != nil {
		if err := ctx.Err(); err != nil {
			return false, err
		}

		return true, nil
	}

	return false, nil
}

func nextPollWait(
	wait time.Duration,
	deadline time.Time,
) time.Duration {
	remaining := time.Until(deadline)
	next := min(wait*2, remaining)

	if next <= 0 {
		next = 10 * time.Millisecond
	}

	return next
}
