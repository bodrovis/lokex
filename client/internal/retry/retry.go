package retry

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"
)

type retryBodyFactory interface {
	// NewBody must return a fresh body for each attempt.
	NewBody() (io.ReadCloser, error)
}

type Config struct {
	Label          string
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
}

// DoWithRetry executes one operation with retries according to cfg.
// If body is seekable or provides a retryBodyFactory, it is reused across
// attempts; otherwise it is buffered into memory once.
func DoWithRetry(
	ctx context.Context,
	cfg Config,
	body io.Reader,
	op func(attempt int, body io.Reader) error,
	isRetryable func(error) bool,
) error {
	if op == nil {
		return fmt.Errorf("retry request: nil op")
	}

	attemptOp, cleanup, err := makeAttemptOp(body, op)
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}

	return WithExpBackoff(
		ctx,
		cfg.Label,
		cfg.MaxRetries,
		cfg.InitialBackoff,
		cfg.MaxBackoff,
		attemptOp,
		isRetryable,
	)
}

func makeAttemptOp(
	body io.Reader,
	op func(attempt int, body io.Reader) error,
) (func(attempt int) error, func(), error) {
	if f, ok := body.(retryBodyFactory); ok {
		return attemptOpFromFactory(f, op), nil, nil
	}

	if rs, ok := body.(io.ReadSeeker); ok {
		attemptOp, cleanup := attemptOpFromReadSeeker(body, rs, op)
		return attemptOp, cleanup, nil
	}

	attemptOp, err := attemptOpFromBufferedBody(body, op)
	return attemptOp, nil, err
}

func attemptOpFromFactory(
	f retryBodyFactory,
	op func(attempt int, body io.Reader) error,
) func(attempt int) error {
	return func(attempt int) error {
		rc, err := f.NewBody()
		if err != nil {
			return fmt.Errorf("create request body: %w", err)
		}
		return op(attempt, rc)
	}
}

func attemptOpFromReadSeeker(
	body io.Reader,
	rs io.ReadSeeker,
	op func(attempt int, body io.Reader) error,
) (func(attempt int) error, func()) {
	if cl, ok := body.(io.Closer); ok {
		cleanup := func() { _ = cl.Close() }

		attemptOp := func(attempt int) error {
			if _, err := rs.Seek(0, io.SeekStart); err != nil {
				return fmt.Errorf("rewind body: %w", err)
			}
			rdr := struct{ io.Reader }{rs} // hide Close
			return op(attempt, rdr)
		}
		return attemptOp, cleanup
	}

	attemptOp := func(attempt int) error {
		if _, err := rs.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("rewind body: %w", err)
		}
		return op(attempt, rs)
	}
	return attemptOp, nil
}

func attemptOpFromBufferedBody(
	body io.Reader,
	op func(attempt int, body io.Reader) error,
) (func(attempt int) error, error) {
	var payload []byte
	if body != nil {
		b, err := io.ReadAll(body)
		if err != nil {
			return nil, fmt.Errorf("buffer request body: %w", err)
		}
		if cbody, ok := body.(io.Closer); ok {
			_ = cbody.Close()
		}
		payload = b
	}

	attemptOp := func(attempt int) error {
		var rdr io.Reader
		if payload != nil {
			rdr = bytes.NewReader(payload)
		}
		return op(attempt, rdr)
	}
	return attemptOp, nil
}
