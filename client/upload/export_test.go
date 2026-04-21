package upload

import (
	"bufio"
	"context"
	"io"
	"os"

	"github.com/bodrovis/lokex/v2/client"
	"github.com/bodrovis/lokex/v2/client/internal/background"
)

func ExportValidateAndNormalizeStdBase64String(s string) (string, error) {
	return validateAndNormalizeStdBase64String(s)
}

func ExportNormalizeStdBase64Padding(s string, pad int) (string, error) {
	return normalizeStdBase64Padding(s, pad)
}

func ExportPollUntilFinished(u *Uploader, ctx context.Context, processID string) (string, error) {
	return u.pollUntilFinished(ctx, processID)
}

func ExportNewUploadBody(ctx context.Context, params UploadParams, cleanPath string) (io.ReadCloser, error) {
	return newUploadBody(ctx, params, cleanPath)
}

func ExportEnsureFileIsRegular(readPath string) error {
	return ensureFileIsRegular(readPath)
}

func ExportSetStatFileForTest(fn func(string) (os.FileInfo, error)) func() {
	prev := statFile
	statFile = fn
	return func() {
		statFile = prev
	}
}

func ExportWriteUploadJSON(w *bufio.Writer, params UploadParams, cleanPath string, spec uploadDataSpec) error {
	return writeUploadJSON(w, params, cleanPath, spec)
}

func ExportWriteUploadKV(w *bufio.Writer, k string, v any, first *bool) error {
	return writeUploadKV(w, k, v, first)
}

func ExportWriteUploadData(w *bufio.Writer, cleanPath string, spec uploadDataSpec) error {
	return writeUploadData(w, cleanPath, spec)
}

func ExportSetOpenFileForTest(fn func(string) (io.ReadCloser, error)) func() {
	prev := openFile
	openFile = fn
	return func() {
		openFile = prev
	}
}

func ExportUploadDataSpecForTest(
	useFile bool,
	dataWasBytes bool,
	dataString string,
	dataBytes []byte,
) uploadDataSpec {
	return uploadDataSpec{
		useFile:      useFile,
		dataWasBytes: dataWasBytes,
		dataString:   dataString,
		dataBytes:    dataBytes,
	}
}

func ExportCloneAndValidateParams(params UploadParams) (UploadParams, string, error) {
	return cloneAndValidateParams(params)
}

func ExportNewUploaderWithClientForTest(c *client.Client) *Uploader {
	return &Uploader{client: c}
}

func ExportKickoffUploadStreaming(
	u *Uploader,
	ctx context.Context,
	body UploadParams,
	cleanPath string,
) (string, error) {
	return u.kickoffUploadStreaming(ctx, body, cleanPath)
}

func ExportSetKickoffUploadStreamingForTest(
	fn func(u *Uploader, ctx context.Context, body UploadParams, cleanPath string) (string, error),
) func() {
	prev := kickoffUploadStreamingFn
	kickoffUploadStreamingFn = fn
	return func() {
		kickoffUploadStreamingFn = prev
	}
}

func ExportUploadBodyFactoryReadForTest() (int, error) {
	var f uploadBodyFactory
	buf := make([]byte, 1)
	return f.Read(buf)
}

func ExportSetBatchUploadSingleForTest(
	fn func(u *Uploader, ctx context.Context, params UploadParams, srcPath string) (string, error),
) func() {
	prev := batchUploadSingleFn
	batchUploadSingleFn = fn
	return func() {
		batchUploadSingleFn = prev
	}
}

func ExportSetBatchUploadConcurrencyForTest(n int) func() {
	prev := batchUploadConcurrency
	batchUploadConcurrency = n
	return func() {
		batchUploadConcurrency = prev
	}
}

type ExportQueuedProcessForTest = background.QueuedProcess

func ExportSetPollProcessesForTest(
	fn func(context.Context, []string, *client.Client) ([]ExportQueuedProcessForTest, error),
) func() {
	prev := pollProcessesFn
	pollProcessesFn = func(ctx context.Context, ids []string, c *client.Client) ([]background.QueuedProcess, error) {
		return fn(ctx, ids, c)
	}
	return func() {
		pollProcessesFn = prev
	}
}

func ExportNewBatchUploadResultItemForTest(
	index int,
	item BatchUploadItem,
) BatchUploadResultItem {
	return newBatchUploadResultItem(index, item)
}

func ExportCollectBatchProcessIDsForTest(
	results []BatchUploadResultItem,
) ([]string, map[string][]int) {
	return collectBatchProcessIDs(results)
}

func ExportMarkBatchPollErrorForTest(
	results []BatchUploadResultItem,
	processIDs []string,
	idToIndexes map[string][]int,
	err error,
) {
	markBatchPollError(results, processIDs, idToIndexes, err)
}

func ExportSetBatchHandleProcessStatusForTest(
	fn func(processID, status, message string) (string, error),
) func() {
	prev := batchHandleProcessStatusFn
	batchHandleProcessStatusFn = fn
	return func() {
		batchHandleProcessStatusFn = prev
	}
}

func ExportPollBatchResultsForTest(
	u *Uploader,
	ctx context.Context,
	results []BatchUploadResultItem,
) {
	u.pollBatchResults(ctx, results)
}

func ExportAcquireBatchUploadSlotForTest(ctx context.Context, sem chan struct{}) error {
	return acquireBatchUploadSlot(ctx, sem)
}

func ExportReleaseBatchUploadSlotForTest(sem chan struct{}) {
	releaseBatchUploadSlot(sem)
}

func ExportCallBatchUploadSingleForTest(
	u *Uploader,
	ctx context.Context,
	params UploadParams,
	srcPath string,
) (string, error) {
	return batchUploadSingleFn(u, ctx, params, srcPath)
}

func ExportKickoffBatchUploadItemForTest(
	u *Uploader,
	ctx context.Context,
	sem chan struct{},
	item BatchUploadItem,
	result *BatchUploadResultItem,
) {
	u.kickoffBatchUploadItem(ctx, sem, item, result)
}
