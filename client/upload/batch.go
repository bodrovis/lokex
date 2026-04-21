package upload

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/bodrovis/lokex/v2/client/internal/background"
)

// batchUploadConcurrency is capped by the Lokalise API.
var batchUploadConcurrency = 6

var batchUploadSingleFn = func(
	u *Uploader,
	ctx context.Context,
	params UploadParams,
	srcPath string,
) (string, error) {
	return u.uploadSingle(ctx, params, srcPath, false)
}

var batchHandleProcessStatusFn = func(processID, status, message string) (string, error) {
	return handleProcessStatus(processID, status, message)
}

// BatchUploadItem describes a single upload job in a batch.
type BatchUploadItem struct {
	Params  UploadParams
	SrcPath string
}

// BatchUploadResultItem contains the result for a single batch item.
// Index always matches the position in the input slice.
type BatchUploadResultItem struct {
	Index     int
	SrcPath   string
	ProcessID string
	Err       error
}

// BatchUploadResult contains per-item results in the same order as input.
type BatchUploadResult struct {
	Items []BatchUploadResultItem
}

// HasErrors reports whether any batch item failed.
func (r BatchUploadResult) HasErrors() bool {
	for _, item := range r.Items {
		if item.Err != nil {
			return true
		}
	}
	return false
}

// SuccessfulProcessIDs returns all process IDs that completed without error.
func (r BatchUploadResult) SuccessfulProcessIDs() []string {
	ids := make([]string, 0, len(r.Items))
	for _, item := range r.Items {
		processID := strings.TrimSpace(item.ProcessID)
		if item.Err == nil && processID != "" {
			ids = append(ids, processID)
		}
	}
	return ids
}

// UploadBatch uploads many files without failing the whole batch on per-file errors.
// Behavior:
//   - Kickoff phase uses uploadSingle(..., poll=false) for each item.
//   - At most 6 uploads are kicked off in parallel (Lokalise API limit).
//   - If poll is false, it returns immediately after kickoff with per-item process IDs/errors.
//   - If poll is true, it polls all successfully-started processes together and records
//     per-item completion errors without discarding successful uploads.
//
// The returned BatchUploadResult always preserves the input order.
// A non-nil error is returned only for fatal batch-level problems (nil client, canceled
// context before start, etc.). Per-item failures are stored in result.Items[i].Err.
func (u *Uploader) UploadBatch(ctx context.Context, items []BatchUploadItem, poll bool) (BatchUploadResult, error) {
	if u == nil || u.client == nil {
		return BatchUploadResult{}, errors.New("upload: batch: uploader/client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return BatchUploadResult{}, err
	}

	results := make([]BatchUploadResultItem, len(items))
	for i, item := range items {
		results[i] = newBatchUploadResultItem(i, item)
	}

	if len(items) == 0 {
		return BatchUploadResult{Items: results}, nil
	}

	u.kickoffBatchUploads(ctx, items, results)

	if poll {
		u.pollBatchResults(ctx, results)
	}

	return BatchUploadResult{Items: results}, nil
}

func newBatchUploadResultItem(index int, item BatchUploadItem) BatchUploadResultItem {
	srcPath := strings.TrimSpace(item.SrcPath)
	if srcPath == "" {
		if filename, ok := item.Params["filename"].(string); ok {
			srcPath = strings.TrimSpace(filename)
		}
	}

	return BatchUploadResultItem{
		Index:   index,
		SrcPath: srcPath,
	}
}

func (u *Uploader) kickoffBatchUploads(ctx context.Context, items []BatchUploadItem, results []BatchUploadResultItem) {
	limit := batchUploadConcurrency
	if limit <= 0 {
		limit = 1
	}

	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup

	for i, item := range items {
		wg.Add(1)

		go func(i int, item BatchUploadItem) {
			defer wg.Done()
			u.kickoffBatchUploadItem(ctx, sem, item, &results[i])
		}(i, item)
	}

	wg.Wait()
}

func (u *Uploader) kickoffBatchUploadItem(
	ctx context.Context,
	sem chan struct{},
	item BatchUploadItem,
	result *BatchUploadResultItem,
) {
	if err := acquireBatchUploadSlot(ctx, sem); err != nil {
		result.Err = err
		return
	}
	defer releaseBatchUploadSlot(sem)

	processID, err := batchUploadSingleFn(u, ctx, item.Params, item.SrcPath)
	result.ProcessID = strings.TrimSpace(processID)
	result.Err = err
}

func releaseBatchUploadSlot(sem chan struct{}) {
	<-sem
}

func acquireBatchUploadSlot(ctx context.Context, sem chan struct{}) error {
	select {
	case sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (u *Uploader) pollBatchResults(ctx context.Context, results []BatchUploadResultItem) {
	processIDs, idToIndexes := collectBatchProcessIDs(results)
	if len(processIDs) == 0 {
		return
	}

	processes, err := pollProcessesFn(ctx, processIDs, u.client)
	if err != nil {
		markBatchPollError(results, processIDs, idToIndexes, fmt.Errorf("upload: poll processes: %w", err))
		return
	}

	applyPolledBatchResults(results, processIDs, idToIndexes, processes)
}

func applyPolledBatchResults(
	results []BatchUploadResultItem,
	processIDs []string,
	idToIndexes map[string][]int,
	processes []background.QueuedProcess,
) {
	seen := make(map[string]bool, len(processes))

	for _, p := range processes {
		processID := strings.TrimSpace(p.ProcessID)
		if processID == "" {
			continue
		}

		indexes, ok := idToIndexes[processID]
		if !ok {
			continue
		}

		seen[processID] = true

		_, err := batchHandleProcessStatusFn(processID, p.Status, p.Message)
		if err != nil {
			markBatchItemError(results, indexes, err)
		}
	}

	markMissingBatchProcessResults(results, processIDs, idToIndexes, seen)
}

func markBatchItemError(results []BatchUploadResultItem, indexes []int, err error) {
	for _, idx := range indexes {
		results[idx].Err = err
	}
}

func markMissingBatchProcessResults(
	results []BatchUploadResultItem,
	processIDs []string,
	idToIndexes map[string][]int,
	seen map[string]bool,
) {
	for _, processID := range processIDs {
		if seen[processID] {
			continue
		}

		err := fmt.Errorf("upload: no process results returned (process_id=%s)", processID)
		for _, idx := range idToIndexes[processID] {
			results[idx].Err = err
		}
	}
}

func collectBatchProcessIDs(results []BatchUploadResultItem) ([]string, map[string][]int) {
	processIDs := make([]string, 0, len(results))
	idToIndexes := make(map[string][]int, len(results))

	for i := range results {
		if results[i].Err != nil {
			continue
		}

		processID := strings.TrimSpace(results[i].ProcessID)
		if processID == "" {
			continue
		}

		if _, exists := idToIndexes[processID]; !exists {
			processIDs = append(processIDs, processID)
		}
		idToIndexes[processID] = append(idToIndexes[processID], i)
	}

	return processIDs, idToIndexes
}

func markBatchPollError(
	results []BatchUploadResultItem,
	processIDs []string,
	idToIndexes map[string][]int,
	err error,
) {
	for _, processID := range processIDs {
		for _, idx := range idToIndexes[processID] {
			results[idx].Err = err
		}
	}
}
