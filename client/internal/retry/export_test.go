package retry

import (
	"io"
	"time"
)

type ExportRetryBodyFactory interface {
	NewBody() (io.ReadCloser, error)
}

func ExportMakeAttemptOp(
	body io.Reader,
	op func(attempt int, body io.Reader) error,
) (func(attempt int) error, func(), error) {
	return makeAttemptOp(body, op)
}

func ExportAttemptOpFromReadSeeker(
	body io.Reader,
	rs io.ReadSeeker,
	op func(attempt int, body io.Reader) error,
) (func(attempt int) error, func()) {
	return attemptOpFromReadSeeker(body, rs, op)
}

func ExportAttemptOpFromBufferedBody(
	body io.Reader,
	op func(attempt int, body io.Reader) error,
) (func(attempt int) error, error) {
	return attemptOpFromBufferedBody(body, op)
}

func ExportResolveRetryable(fn func(error) bool) func(error) bool {
	return resolveRetryable(fn)
}

func ExportComputeRetryDelay(backoff, maxBackoff time.Duration) time.Duration {
	return computeRetryDelay(backoff, maxBackoff)
}

func ExportWrapErr(label string, attempt, total int, err error) error {
	return wrapErr(label, attempt, total, err)
}

func ExportWrapCtxErr(label string, attempt, total int, err error) error {
	return wrapCtxErr(label, attempt, total, err)
}

func ExportSetJitteredBackoffForTest(fn func(time.Duration) time.Duration) func() {
	prev := jitteredBackoff
	jitteredBackoff = fn
	return func() {
		jitteredBackoff = prev
	}
}
