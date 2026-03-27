package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bodrovis/lokex/v2/internal/apierr"
	"github.com/bodrovis/lokex/v2/internal/utils"
	"golang.org/x/sync/errgroup"
)

const (
	// Queued process statuses
	StatusQueued   = "queued"
	StatusFinished = "finished"
	StatusFailed   = "failed"
)

// QueuedProcess is a normalized view over Lokalise "processes/*" responses.
// DownloadURL is populated when the process produces a file (e.g., download).
type QueuedProcess struct {
	ProcessID   string `json:"process_id"`
	Status      string `json:"status"`
	DownloadURL string `json:"download_url,omitempty"`
	Message     string `json:"message,omitempty"`
}

// processResponse mirrors the subset of the Lokalise response we care about.
// It stays unexported; callers use QueuedProcess instead.
type processResponse struct {
	Process struct {
		ProcessID string `json:"process_id"`
		Status    string `json:"status"`
		Message   string `json:"message"`
		Details   struct {
			DownloadURL string `json:"download_url"`
		} `json:"details"`
	} `json:"process"`
}

type pollResult struct {
	id   string
	proc QueuedProcess
	err  error
}

// ToQueuedProcess converts a typed API response into a flattened QueuedProcess.
func (pr *processResponse) ToQueuedProcess() QueuedProcess {
	return QueuedProcess{
		ProcessID:   pr.Process.ProcessID,
		Status:      utils.NormalizeString(pr.Process.Status),
		Message:     strings.TrimSpace(pr.Process.Message),
		DownloadURL: pr.Process.Details.DownloadURL,
	}
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
//   - We enforce an overall deadline via context.WithDeadline.
func (c *Client) PollProcesses(ctx context.Context, processIDs []string) ([]QueuedProcess, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	wait, maxWait := c.pollConfig()
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
		procs, errs := c.pollRound(pollCtx, pending, maxConcurrent)

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

// pollConfig returns initial wait and overall max wait with safe defaults.
func (c *Client) pollConfig() (wait time.Duration, maxWait time.Duration) {
	// This might be an overkill as config should already
	// check these values but let's leave just in case.
	wait = c.PollInitialWait
	if wait <= 0 {
		wait = defaultPollInitialWait
	}

	maxWait = c.PollMaxWait
	if maxWait <= 0 {
		maxWait = defaultPollMaxWait
	}
	if maxWait < wait {
		maxWait = wait
	}
	return wait, maxWait
}

// normalizeProcessIDs trims inputs, preserves caller order (including duplicates),
// and returns:
//   - ordered: trimmed IDs in original order (empties kept as "")
//   - processMap: latest status per UNIQUE non-empty ID (seeded with StatusQueued)
//   - pending: set of UNIQUE non-empty IDs to poll
func normalizeProcessIDs(processIDs []string) (ordered []string, processMap map[string]QueuedProcess, pending map[string]struct{}) {
	ordered = make([]string, 0, len(processIDs))
	processMap = make(map[string]QueuedProcess, len(processIDs))
	pending = make(map[string]struct{}, len(processIDs))

	for _, raw := range processIDs {
		id := strings.TrimSpace(raw)
		ordered = append(ordered, id)
		if id == "" {
			continue
		}
		if _, ok := processMap[id]; !ok {
			processMap[id] = QueuedProcess{ProcessID: id, Status: StatusQueued}
		}
		pending[id] = struct{}{}
	}
	return
}

// pollRound performs one polling round for all currently pending IDs.
// It returns successful process statuses and per-ID errors.
// Workers never block on send because resCh is buffered to len(pending).
func (c *Client) pollRound(ctx context.Context, pending map[string]struct{}, maxConcurrent int) ([]QueuedProcess, map[string]error) {
	ids := make([]string, 0, len(pending))
	for id := range pending {
		ids = append(ids, id)
	}

	resCh := make(chan pollResult, len(ids))

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrent)

	for _, id := range ids {
		cur := id
		g.Go(func() error {
			path := c.projectPath(fmt.Sprintf("processes/%s", cur))
			var resp processResponse
			if err := c.doRequest(gctx, http.MethodGet, path, nil, &resp, nil); err != nil {
				resCh <- pollResult{id: cur, err: err}
				return nil
			}
			resCh <- pollResult{id: cur, proc: resp.ToQueuedProcess()}
			return nil
		})
	}

	_ = g.Wait()
	close(resCh)

	procs := make([]QueuedProcess, 0, len(ids))
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
	// Successful statuses update the latest view and remove terminal IDs.
	for _, p := range procs {
		processMap[p.ProcessID] = p
		if p.Status == StatusFinished || p.Status == StatusFailed {
			delete(pending, p.ProcessID)
		}
	}

	// Errors: retryable stays pending; non-retryable fails and is removed.
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

// buildResults reconstructs output preserving caller order and duplicates.
// Empty IDs are skipped.
func buildResults(ordered []string, processMap map[string]QueuedProcess) []QueuedProcess {
	out := make([]QueuedProcess, 0, len(ordered))
	for _, id := range ordered {
		if id == "" {
			continue
		}
		if p, ok := processMap[id]; ok {
			out = append(out, p)
		} else {
			out = append(out, QueuedProcess{ProcessID: id, Status: StatusQueued})
		}
	}
	return out
}

// projectPath builds "projects/{id}/<suffix>" for project-scoped endpoints.
func (c *Client) projectPath(suffix string) string {
	return fmt.Sprintf("projects/%s/%s", url.PathEscape(c.ProjectID), suffix)
}
