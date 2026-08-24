package retry_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/bodrovis/lokex/v2/client/internal/retry"
)

type testRetryBodyFactory struct {
	newBody func() (io.ReadCloser, error)
}

func (f testRetryBodyFactory) Read(_ []byte) (int, error) {
	return 0, io.EOF
}

func (f testRetryBodyFactory) NewBody() (io.ReadCloser, error) {
	return f.newBody()
}

type errReadSeekCloser struct {
	seekErr error
	closed  *bool
}

func (r *errReadSeekCloser) Read(_ []byte) (int, error) {
	return 0, io.EOF
}

func (r *errReadSeekCloser) Seek(_ int64, _ int) (int64, error) {
	return 0, r.seekErr
}

func (r *errReadSeekCloser) Close() error {
	if r.closed != nil {
		*r.closed = true
	}
	return nil
}

type testReadSeekCloser struct {
	*strings.Reader
	closed *bool
}

func (r *testReadSeekCloser) Close() error {
	if r.closed != nil {
		*r.closed = true
	}
	return nil
}

type errReadSeeker struct {
	seekErr error
}

func (r *errReadSeeker) Read(_ []byte) (int, error) {
	return 0, io.EOF
}

func (r *errReadSeeker) Seek(_ int64, _ int) (int64, error) {
	return 0, r.seekErr
}

type errReader struct {
	err error
}

func (r errReader) Read(_ []byte) (int, error) {
	return 0, r.err
}

type closeTrackingReader struct {
	r      io.Reader
	closed *bool
}

func (r *closeTrackingReader) Read(p []byte) (int, error) {
	return r.r.Read(p)
}

func (r *closeTrackingReader) Close() error {
	if r.closed != nil {
		*r.closed = true
	}
	return nil
}

type plainReader struct {
	r io.Reader
}

func (r plainReader) Read(p []byte) (int, error) {
	return r.r.Read(p)
}

func TestDoWithRetry(t *testing.T) {
	t.Parallel()

	t.Run("nil op", func(t *testing.T) {
		t.Parallel()

		err := retry.DoWithRetry(
			context.Background(),
			retry.Config{},
			nil,
			nil,
			nil,
		)
		if err == nil {
			t.Fatal("DoWithRetry() error = nil, want non-nil")
		}
		if err.Error() != "retry request: nil op" {
			t.Fatalf("error = %q, want %q", err.Error(), "retry request: nil op")
		}
	})

	t.Run("closes body when buffering fails", func(t *testing.T) {
		t.Parallel()

		closed := false
		readErr := errors.New("read boom")

		body := &closeTrackingReader{
			r:      errReader{err: readErr},
			closed: &closed,
		}

		attemptOp, err := retry.ExportAttemptOpFromBufferedBody(
			body,
			func(int, io.Reader) error { return nil },
		)

		if !errors.Is(err, readErr) {
			t.Fatalf("error = %v, want %v", err, readErr)
		}
		if attemptOp != nil {
			t.Fatal("attemptOp != nil, want nil")
		}
		if !closed {
			t.Fatal("body was not closed after buffering failed")
		}
	})

	t.Run("factory body creation error is returned", func(t *testing.T) {
		t.Parallel()

		bodyErr := errors.New("cannot create body")

		body := testRetryBodyFactory{
			newBody: func() (io.ReadCloser, error) {
				return nil, bodyErr
			},
		}

		attemptOp, cleanup, err := retry.ExportMakeAttemptOp(
			body,
			func(int, io.Reader) error { return nil },
		)
		if err != nil {
			t.Fatalf("MakeAttemptOp() unexpected error = %v", err)
		}
		if cleanup != nil {
			t.Fatal("cleanup != nil, want nil")
		}

		err = attemptOp(0)
		if !errors.Is(err, bodyErr) {
			t.Fatalf("error = %v, want %v", err, bodyErr)
		}
		if err.Error() != "create request body: cannot create body" {
			t.Fatalf("error = %q", err.Error())
		}
	})

	t.Run("read seeker is rewound for each retry and closed once", func(t *testing.T) {
		t.Parallel()

		closed := false
		body := &testReadSeekCloser{
			Reader: strings.NewReader("payload"),
			closed: &closed,
		}

		attempts := 0

		err := retry.DoWithRetry(
			context.Background(),
			retry.Config{
				MaxRetries:     1,
				InitialBackoff: time.Nanosecond,
				MaxBackoff:     time.Nanosecond,
			},
			body,
			func(attempt int, rdr io.Reader) error {
				attempts++

				b, err := io.ReadAll(rdr)
				if err != nil {
					return err
				}
				if string(b) != "payload" {
					t.Fatalf("attempt %d body = %q, want %q", attempt, b, "payload")
				}

				if attempt == 0 {
					return errors.New("retry me")
				}
				return nil
			},
			func(error) bool { return true },
		)

		if err != nil {
			t.Fatalf("DoWithRetry() error = %v", err)
		}
		if attempts != 2 {
			t.Fatalf("attempts = %d, want 2", attempts)
		}
		if !closed {
			t.Fatal("body was not closed")
		}
	})

	t.Run("makeAttemptOp error is returned", func(t *testing.T) {
		t.Parallel()

		readErr := errors.New("boom")

		err := retry.DoWithRetry(
			context.Background(),
			retry.Config{
				Label:          "test",
				MaxRetries:     0,
				InitialBackoff: time.Millisecond,
				MaxBackoff:     time.Millisecond,
			},
			errReader{err: readErr},
			func(_ int, _ io.Reader) error { return nil },
			nil,
		)
		if err == nil {
			t.Fatal("DoWithRetry() error = nil, want non-nil")
		}
		if err.Error() != "buffer request body: boom" {
			t.Fatalf("error = %q, want %q", err.Error(), "buffer request body: boom")
		}
	})

	t.Run("cleanup is called for read seeker closer", func(t *testing.T) {
		t.Parallel()

		closed := false
		body := &testReadSeekCloser{
			Reader: strings.NewReader("payload"),
			closed: &closed,
		}

		var gotBody string
		err := retry.DoWithRetry(
			context.Background(),
			retry.Config{
				Label:          "test",
				MaxRetries:     0,
				InitialBackoff: time.Millisecond,
				MaxBackoff:     time.Millisecond,
			},
			body,
			func(_ int, rdr io.Reader) error {
				b, err := io.ReadAll(rdr)
				if err != nil {
					return err
				}
				gotBody = string(b)
				return nil
			},
			nil,
		)
		if err != nil {
			t.Fatalf("DoWithRetry() unexpected error = %v", err)
		}
		if gotBody != "payload" {
			t.Fatalf("body = %q, want %q", gotBody, "payload")
		}
		if !closed {
			t.Fatal("cleanup was not called, want body to be closed")
		}
	})
}

func TestMakeAttemptOp(t *testing.T) {
	t.Parallel()

	t.Run("uses retry body factory", func(t *testing.T) {
		t.Parallel()

		factoryCalls := 0

		body := testRetryBodyFactory{
			newBody: func() (io.ReadCloser, error) {
				factoryCalls++
				return io.NopCloser(strings.NewReader("factory-body")), nil
			},
		}

		attemptOp, cleanup, err := retry.ExportMakeAttemptOp(
			body,
			func(_ int, rdr io.Reader) error {
				b, err := io.ReadAll(rdr)
				if err != nil {
					return err
				}
				if string(b) != "factory-body" {
					t.Fatalf("body = %q, want %q", string(b), "factory-body")
				}
				return nil
			},
		)
		if err != nil {
			t.Fatalf("MakeAttemptOp() unexpected error = %v", err)
		}
		if cleanup != nil {
			t.Fatal("cleanup != nil, want nil for factory body")
		}

		if err := attemptOp(1); err != nil {
			t.Fatalf("attemptOp() unexpected error = %v", err)
		}
		if factoryCalls != 1 {
			t.Fatalf("factory calls = %d, want %d", factoryCalls, 1)
		}
	})

	t.Run("uses buffered body fallback", func(t *testing.T) {
		t.Parallel()

		body := plainReader{
			r: strings.NewReader("buffered-body"),
		}

		attemptOp, cleanup, err := retry.ExportMakeAttemptOp(
			body,
			func(_ int, rdr io.Reader) error {
				b, err := io.ReadAll(rdr)
				if err != nil {
					return err
				}

				if string(b) != "buffered-body" {
					t.Fatalf("body = %q, want %q", string(b), "buffered-body")
				}

				return nil
			},
		)

		if err != nil {
			t.Fatalf("MakeAttemptOp() unexpected error = %v", err)
		}

		if cleanup != nil {
			t.Fatal("cleanup != nil, want nil for buffered body")
		}

		if err := attemptOp(1); err != nil {
			t.Fatalf("attemptOp() unexpected error = %v", err)
		}
	})

	t.Run("buffered body error is returned", func(t *testing.T) {
		t.Parallel()

		readErr := errors.New("read failed")

		attemptOp, cleanup, err := retry.ExportMakeAttemptOp(
			errReader{err: readErr},
			func(_ int, _ io.Reader) error { return nil },
		)
		if err == nil {
			t.Fatal("MakeAttemptOp() error = nil, want non-nil")
		}
		if err.Error() != "buffer request body: read failed" {
			t.Fatalf("error = %q, want %q", err.Error(), "buffer request body: read failed")
		}
		if attemptOp != nil {
			t.Fatal("attemptOp != nil, want nil on error")
		}
		if cleanup != nil {
			t.Fatal("cleanup != nil, want nil on error")
		}
	})
}

func TestAttemptOpFromReadSeeker(t *testing.T) {
	t.Parallel()

	t.Run("read seeker closer hides close and returns cleanup", func(t *testing.T) {
		t.Parallel()

		closed := false
		body := &testReadSeekCloser{
			Reader: strings.NewReader("abc"),
			closed: &closed,
		}

		var gotBody string
		var gotCloser bool

		attemptOp, cleanup := retry.ExportAttemptOpFromReadSeeker(
			body,
			body,
			func(_ int, rdr io.Reader) error {
				_, gotCloser = rdr.(io.Closer)

				b, err := io.ReadAll(rdr)
				if err != nil {
					return err
				}
				gotBody = string(b)
				return nil
			},
		)

		if cleanup == nil {
			t.Fatal("cleanup = nil, want non-nil")
		}
		if err := attemptOp(1); err != nil {
			t.Fatalf("attemptOp() unexpected error = %v", err)
		}
		if gotBody != "abc" {
			t.Fatalf("body = %q, want %q", gotBody, "abc")
		}
		if gotCloser {
			t.Fatal("body passed to op implements io.Closer, want Close to be hidden")
		}

		cleanup()
		if !closed {
			t.Fatal("cleanup did not close body")
		}
	})

	t.Run("seek error from read seeker closer", func(t *testing.T) {
		t.Parallel()

		seekErr := errors.New("cannot seek")
		rs := &errReadSeeker{seekErr: seekErr}

		attemptOp, cleanup := retry.ExportAttemptOpFromReadSeeker(
			rs,
			rs,
			func(_ int, _ io.Reader) error { return nil },
		)
		if cleanup != nil {
			t.Fatal("cleanup != nil, want nil")
		}

		err := attemptOp(1)
		if err == nil {
			t.Fatal("attemptOp() error = nil, want non-nil")
		}
		if err.Error() != "rewind body: cannot seek" {
			t.Fatalf("error = %q, want %q", err.Error(), "rewind body: cannot seek")
		}
	})

	t.Run("seek error from read seeker closer branch", func(t *testing.T) {
		t.Parallel()

		seekErr := errors.New("boom")
		closed := false

		body := &errReadSeekCloser{
			seekErr: seekErr,
			closed:  &closed,
		}

		attemptOp, cleanup := retry.ExportAttemptOpFromReadSeeker(
			body,
			body,
			func(_ int, _ io.Reader) error { return nil },
		)

		if cleanup == nil {
			t.Fatal("cleanup = nil, want non-nil")
		}

		err := attemptOp(1)
		if err == nil {
			t.Fatal("attemptOp() error = nil, want non-nil")
		}
		if err.Error() != "rewind body: boom" {
			t.Fatalf("error = %q, want %q", err.Error(), "rewind body: boom")
		}

		cleanup()
		if !closed {
			t.Fatal("cleanup did not close body")
		}
	})
}

func TestAttemptOpFromBufferedBody(t *testing.T) {
	t.Parallel()

	t.Run("nil body passes nil reader", func(t *testing.T) {
		t.Parallel()

		attemptOp, err := retry.ExportAttemptOpFromBufferedBody(
			nil,
			func(_ int, rdr io.Reader) error {
				if rdr != nil {
					t.Fatal("reader != nil, want nil")
				}
				return nil
			},
		)
		if err != nil {
			t.Fatalf("AttemptOpFromBufferedBody() unexpected error = %v", err)
		}

		if err := attemptOp(1); err != nil {
			t.Fatalf("attemptOp() unexpected error = %v", err)
		}
	})

	t.Run("buffers body and closes closer", func(t *testing.T) {
		t.Parallel()

		closed := false
		body := &closeTrackingReader{
			r:      strings.NewReader("buffer-me"),
			closed: &closed,
		}

		attemptOp, err := retry.ExportAttemptOpFromBufferedBody(
			body,
			func(_ int, rdr io.Reader) error {
				b, err := io.ReadAll(rdr)
				if err != nil {
					return err
				}
				if string(b) != "buffer-me" {
					t.Fatalf("body = %q, want %q", string(b), "buffer-me")
				}
				return nil
			},
		)
		if err != nil {
			t.Fatalf("AttemptOpFromBufferedBody() unexpected error = %v", err)
		}
		if !closed {
			t.Fatal("body was not closed after buffering")
		}

		if err := attemptOp(1); err != nil {
			t.Fatalf("attemptOp() unexpected error = %v", err)
		}
		if err := attemptOp(2); err != nil {
			t.Fatalf("attemptOp() unexpected error on second call = %v", err)
		}
	})

	t.Run("read error while buffering", func(t *testing.T) {
		t.Parallel()

		attemptOp, err := retry.ExportAttemptOpFromBufferedBody(
			errReader{err: errors.New("read boom")},
			func(_ int, _ io.Reader) error { return nil },
		)
		if err == nil {
			t.Fatal("AttemptOpFromBufferedBody() error = nil, want non-nil")
		}
		if err.Error() != "buffer request body: read boom" {
			t.Fatalf("error = %q, want %q", err.Error(), "buffer request body: read boom")
		}
		if attemptOp != nil {
			t.Fatal("attemptOp != nil, want nil on error")
		}
	})
}
