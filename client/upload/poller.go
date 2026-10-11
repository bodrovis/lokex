package upload

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/bodrovis/lokex/v2/client/internal/background"
)

var pollProcessesFn = background.PollProcesses

// pollUntilFinished polls a single process until it reaches a terminal status.
// It returns the process ID on "finished" and an error otherwise.
func (u *Uploader) pollUntilFinished(ctx context.Context, processID string) (string, error) {
	processID = strings.TrimSpace(processID)
	if processID == "" {
		return "", errors.New("upload: empty process_id")
	}

	results, err := pollProcessesFn(ctx, []string{processID}, u.client)
	if err != nil {
		return "", fmt.Errorf("upload: poll processes: %w", err)
	}
	if len(results) == 0 {
		return "", fmt.Errorf("upload: no process results returned (process_id=%s)", processID)
	}

	return handleProcessResult(processID, results[0])
}

func handleProcessStatus(processID, status, message string) (string, error) {
	st := strings.ToLower(strings.TrimSpace(status))

	switch st {
	case background.StatusFinished:
		return processID, nil

	case background.StatusFailed, background.StatusCancelled:
		if msg := strings.TrimSpace(message); msg != "" {
			return "", fmt.Errorf(
				"upload: process %s %s: %s",
				processID, st, msg,
			)
		}
		return "", fmt.Errorf("upload: process %s %s", processID, st)

	default:
		return "", fmt.Errorf(
			"upload: process %s did not finish (status=%q)",
			processID, st,
		)
	}
}

func handleProcessResult(
	processID string,
	p background.QueuedProcess,
) (string, error) {
	if p.Err != nil {
		return "", fmt.Errorf(
			"upload: process %s polling failed: %w",
			processID,
			p.Err,
		)
	}

	return handleProcessStatus(processID, p.Status, p.Message)
}
