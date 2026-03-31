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

func ExportNewStoppedTimer() *time.Timer {
	return newStoppedTimer()
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

func ExportSetNewTimerForTest(fn func(time.Duration) *time.Timer) func() {
	prev := newTimer
	newTimer = fn
	return func() {
		newTimer = prev
	}
}

func ExportSetSleepWithTimerForTest(
	fn func(context.Context, *time.Timer, time.Duration) error,
) func() {
	prev := sleepWithTimer
	sleepWithTimer = fn
	return func() {
		sleepWithTimer = prev
	}
}

func ExportSetNextSleepWaitForTest(
	fn func(time.Duration, time.Time) (time.Duration, bool),
) func() {
	prev := nextSleepWaitFn
	nextSleepWaitFn = fn
	return func() {
		nextSleepWaitFn = prev
	}
}

func ExportNextPollWait(wait time.Duration, deadline time.Time) time.Duration {
	return nextPollWait(wait, deadline)
}
