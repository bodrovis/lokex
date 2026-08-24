package retry

import (
	"io"
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
