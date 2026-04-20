package upload

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/bodrovis/lokex/v2/client"
	"github.com/bodrovis/lokex/v2/client/internal/background"
	"github.com/bodrovis/lokex/v2/internal/utils"
)

var pollProcessesFn = func(
	ctx context.Context,
	processIDs []string,
	c *client.Client,
) ([]background.QueuedProcess, error) {
	return background.PollProcesses(ctx, processIDs, c)
}

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

	p := results[0]
	return handleProcessStatus(processID, p.Status, p.Message)
}

func handleProcessStatus(processID, status, message string) (string, error) {
	st := utils.NormalizeString(status)

	switch st {
	case background.StatusFinished:
		return processID, nil

	case background.StatusFailed:
		if msg := strings.TrimSpace(message); msg != "" {
			return "", fmt.Errorf("upload: process %s failed: %s", processID, msg)
		}
		return "", fmt.Errorf("upload: process %s failed", processID)

	default:
		return "", fmt.Errorf("upload: process %s did not finish (status=%q)", processID, st)
	}
}
