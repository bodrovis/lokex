package background

import (
	"context"
	"time"

	"github.com/bodrovis/lokex/v2/client"
)

func ExportBuildResults(ordered []string, processMap map[string]QueuedProcess) []QueuedProcess {
	return buildResults(ordered, processMap)
}

func ExportApplyRound(
	processMap map[string]QueuedProcess,
	pending map[string]struct{},
	procs []QueuedProcess,
	errs map[string]error,
) {
	applyRound(processMap, pending, procs, errs)
}

func ExportNextSleepWait(wait time.Duration, deadline time.Time) (time.Duration, bool) {
	return nextSleepWait(wait, deadline)
}

func ExportSleepBetweenPollRounds(
	ctx context.Context,
	pollCtx context.Context,
	timer *time.Timer,
	sleep time.Duration,
) (bool, error) {
	return sleepBetweenPollRounds(ctx, pollCtx, timer, sleep)
}

func ExportSetPollRoundForTest(
	fn func(context.Context, *client.Client, map[string]struct{}, int) ([]QueuedProcess, map[string]error),
) func() {
	prev := pollRoundFn
	pollRoundFn = fn
	return func() {
		pollRoundFn = prev
	}
}

func ExportNextPollWait(wait time.Duration, deadline time.Time) time.Duration {
	return nextPollWait(wait, deadline)
}
