package upload_test

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bodrovis/lokex/v2/client/upload"
)

type errWriter struct {
	writeErr error
}

func (w errWriter) Write(_ []byte) (int, error) {
	return 0, w.writeErr
}

type failAfterNWriter struct {
	n     int
	wrote int
	err   error
}

func (w *failAfterNWriter) Write(p []byte) (int, error) {
	if w.wrote >= w.n {
		return 0, w.err
	}

	remaining := w.n - w.wrote
	if len(p) > remaining {
		w.wrote += remaining
		return remaining, w.err
	}

	w.wrote += len(p)
	return len(p), nil
}

type fakeReadCloser struct {
	r        io.Reader
	closeErr error
}

func (f *fakeReadCloser) Read(p []byte) (int, error) {
	return f.r.Read(p)
}

func (f *fakeReadCloser) Close() error {
	return f.closeErr
}

func TestWriteUploadJSON(t *testing.T) {
	t.Run("write opening brace error", func(t *testing.T) {
		t.Parallel()

		bw := bufio.NewWriterSize(errWriter{writeErr: errors.New("boom")}, 1)
		err := upload.ExportWriteUploadJSON(
			bw,
			upload.UploadParams{},
			"",
			upload.ExportUploadDataSpecForTest(false, false, "abc", nil),
		)
		if err == nil {
			t.Fatal("WriteUploadJSON() error = nil, want non-nil")
		}
		if err.Error() != "boom" {
			t.Fatalf("error = %q, want %q", err.Error(), "boom")
		}
	})

	t.Run("write comma before data error", func(t *testing.T) {
		t.Parallel()

		w := &failAfterNWriter{n: 1, err: errors.New("comma boom")}
		bw := bufio.NewWriterSize(w, 1)

		err := upload.ExportWriteUploadJSON(
			bw,
			upload.UploadParams{},
			"",
			upload.ExportUploadDataSpecForTest(false, false, "abc", nil),
		)
		if err == nil {
			t.Fatal("WriteUploadJSON() error = nil, want non-nil")
		}
		if err.Error() != "comma boom" {
			t.Fatalf("error = %q, want %q", err.Error(), "comma boom")
		}
	})

	t.Run("write data prefix error", func(t *testing.T) {
		t.Parallel()

		w := &failAfterNWriter{n: 2, err: errors.New("prefix boom")}
		bw := bufio.NewWriterSize(w, 1)

		err := upload.ExportWriteUploadJSON(
			bw,
			upload.UploadParams{},
			"",
			upload.ExportUploadDataSpecForTest(false, false, "abc", nil),
		)
		if err == nil {
			t.Fatal("WriteUploadJSON() error = nil, want non-nil")
		}
		if err.Error() != "prefix boom" {
			t.Fatalf("error = %q, want %q", err.Error(), "prefix boom")
		}
	})

	t.Run("success with params and inline data", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		bw := bufio.NewWriter(&buf)

		err := upload.ExportWriteUploadJSON(
			bw,
			upload.UploadParams{
				"lang_iso": "en",
				"replace":  true,
				"data":     "ignored-by-loop",
			},
			"",
			upload.ExportUploadDataSpecForTest(false, false, "YWJj", nil),
		)
		if err != nil {
			t.Fatalf("WriteUploadJSON() unexpected error = %v", err)
		}
		if err := bw.Flush(); err != nil {
			t.Fatalf("Flush() error = %v", err)
		}

		got := buf.String()
		if !strings.HasPrefix(got, "{") || !strings.HasSuffix(got, `"}`) {
			t.Fatalf("json = %q, want object ending with data field", got)
		}
		if !strings.Contains(got, `"data":"YWJj"`) {
			t.Fatalf("json = %q, want embedded data field", got)
		}
		if !strings.Contains(got, `"lang_iso":"en"`) {
			t.Fatalf("json = %q, want lang_iso", got)
		}
		if !strings.Contains(got, `"replace":true`) {
			t.Fatalf("json = %q, want replace=true", got)
		}
	})

	t.Run("array param is encoded", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		bw := bufio.NewWriter(&buf)

		err := upload.ExportWriteUploadJSON(
			bw,
			upload.UploadParams{
				"lang_iso": "en",
				"tags":     []string{"foo", "bar"},
			},
			"",
			upload.ExportUploadDataSpecForTest(false, false, "YWJj", nil),
		)
		if err != nil {
			t.Fatalf("WriteUploadJSON() unexpected error = %v", err)
		}

		if err := bw.Flush(); err != nil {
			t.Fatalf("Flush() error = %v", err)
		}

		if !strings.Contains(buf.String(), `"tags":["foo","bar"]`) {
			t.Fatalf("json = %q, want tags array", buf.String())
		}
	})

	t.Run("nil slice param is encoded as empty array", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		bw := bufio.NewWriter(&buf)

		err := upload.ExportWriteUploadJSON(
			bw,
			upload.UploadParams{
				"tags": []string(nil),
			},
			"",
			upload.ExportUploadDataSpecForTest(false, false, "YWJj", nil),
		)
		if err != nil {
			t.Fatalf("WriteUploadJSON() unexpected error = %v", err)
		}

		if err := bw.Flush(); err != nil {
			t.Fatalf("Flush() error = %v", err)
		}

		if !strings.Contains(buf.String(), `"tags":[]`) {
			t.Fatalf("json = %q, want empty tags array", buf.String())
		}
	})
}

func TestWriteUploadData(t *testing.T) {
	t.Run("inline base64 string is written as is", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		bw := bufio.NewWriter(&buf)

		err := upload.ExportWriteUploadData(
			bw,
			"",
			upload.ExportUploadDataSpecForTest(false, false, "YWJj", nil),
		)
		if err != nil {
			t.Fatalf("WriteUploadData() unexpected error = %v", err)
		}
		if err := bw.Flush(); err != nil {
			t.Fatalf("Flush() error = %v", err)
		}
		if buf.String() != "YWJj" {
			t.Fatalf("got = %q, want %q", buf.String(), "YWJj")
		}
	})

	t.Run("file open error", func(t *testing.T) {
		t.Parallel()

		missing := filepath.Join(t.TempDir(), "missing.txt")

		var buf bytes.Buffer
		bw := bufio.NewWriter(&buf)

		err := upload.ExportWriteUploadData(
			bw,
			missing,
			upload.ExportUploadDataSpecForTest(true, false, "", nil),
		)

		if err == nil {
			t.Fatal("WriteUploadData() error = nil, want non-nil")
		}

		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("error = %v, want os.ErrNotExist", err)
		}
	})

	t.Run("bytes path", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		bw := bufio.NewWriter(&buf)

		err := upload.ExportWriteUploadData(
			bw,
			"",
			upload.ExportUploadDataSpecForTest(false, true, "", []byte("abc")),
		)
		if err != nil {
			t.Fatalf("WriteUploadData() unexpected error = %v", err)
		}
		if err := bw.Flush(); err != nil {
			t.Fatalf("Flush() error = %v", err)
		}
		if buf.String() != "YWJj" {
			t.Fatalf("got = %q, want %q", buf.String(), "YWJj")
		}
	})

	t.Run("copy error is returned", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "file.txt")
		if err := os.WriteFile(path, []byte("abc"), 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		writeErr := errors.New("write boom")
		w := &failAfterNWriter{
			n:   1,
			err: writeErr,
		}
		bw := bufio.NewWriterSize(w, 1)

		err := upload.ExportWriteUploadData(
			bw,
			path,
			upload.ExportUploadDataSpecForTest(true, false, "", nil),
		)

		if err == nil {
			t.Fatal("WriteUploadData() error = nil, want non-nil")
		}

		if !errors.Is(err, writeErr) {
			t.Fatalf("error = %v, want write error", err)
		}
	})

	t.Run("encoder close error is returned", func(t *testing.T) {
		t.Parallel()

		w := &failAfterNWriter{n: 0, err: errors.New("flush boom")}
		bw := bufio.NewWriterSize(w, 1)

		err := upload.ExportWriteUploadData(
			bw,
			"",
			upload.ExportUploadDataSpecForTest(false, true, "", []byte("a")),
		)
		if err == nil {
			t.Fatal("WriteUploadData() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "flush boom") {
			t.Fatalf("error = %q, want encoder close error", err.Error())
		}
	})
}

func TestWriteUploadJSON_ErrorPaths(t *testing.T) {
	t.Run("opening brace write error", func(t *testing.T) {
		t.Parallel()

		w := &failAfterNWriter{n: 0, err: errors.New("brace boom")}
		bw := bufio.NewWriterSize(w, 1)

		// Fill the internal buffer so the next write triggers a flush.
		if _, err := bw.WriteString("x"); err != nil {
			t.Fatalf("prefill write error = %v", err)
		}

		err := upload.ExportWriteUploadJSON(
			bw,
			upload.UploadParams{},
			"",
			upload.ExportUploadDataSpecForTest(false, false, "YWJj", nil),
		)
		if err == nil {
			t.Fatal("WriteUploadJSON() error = nil, want non-nil")
		}
		if err.Error() != "brace boom" {
			t.Fatalf("error = %q, want %q", err.Error(), "brace boom")
		}
	})

	t.Run("writeUploadKV error is returned", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		bw := bufio.NewWriter(&buf)

		err := upload.ExportWriteUploadJSON(
			bw,
			upload.UploadParams{
				"bad": func() {},
			},
			"",
			upload.ExportUploadDataSpecForTest(false, false, "YWJj", nil),
		)
		if err == nil {
			t.Fatal("WriteUploadJSON() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "marshal from Go func()") {
			t.Fatalf("error = %q, want marshal error", err.Error())
		}
	})

	t.Run("writeComma error is returned", func(t *testing.T) {
		t.Parallel()

		// With buffer size 1:
		// - "{" gets flushed later
		// - `"a"` writes through
		// - ":" gets flushed by writing value
		// - "1" stays buffered
		// - writeComma() flushes buffered "1" and hits this error
		w := &failAfterNWriter{n: 5, err: errors.New("comma boom")}
		bw := bufio.NewWriterSize(w, 1)

		err := upload.ExportWriteUploadJSON(
			bw,
			upload.UploadParams{
				"a": 1,
			},
			"",
			upload.ExportUploadDataSpecForTest(false, false, "YWJj", nil),
		)
		if err == nil {
			t.Fatal("WriteUploadJSON() error = nil, want non-nil")
		}
		if err.Error() != "comma boom" {
			t.Fatalf("error = %q, want %q", err.Error(), "comma boom")
		}
	})
}

func TestWriteUploadKV_ErrorPaths(t *testing.T) {
	t.Run("comma write error when not first", func(t *testing.T) {
		t.Parallel()

		w := &failAfterNWriter{n: 0, err: errors.New("comma boom")}
		bw := bufio.NewWriterSize(w, 1)

		// Fill the buffer so the comma write triggers a flush.
		if _, err := bw.WriteString("x"); err != nil {
			t.Fatalf("prefill write error = %v", err)
		}

		first := false
		err := upload.ExportWriteUploadKV(bw, "k", "v", &first)
		if err == nil {
			t.Fatal("WriteUploadKV() error = nil, want non-nil")
		}
		if err.Error() != "comma boom" {
			t.Fatalf("error = %q, want %q", err.Error(), "comma boom")
		}
	})

	t.Run("value marshal error", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		bw := bufio.NewWriter(&buf)

		first := true
		err := upload.ExportWriteUploadKV(bw, "k", func() {}, &first)

		if err == nil {
			t.Fatal("WriteUploadKV() error = nil, want non-nil")
		}

		if !strings.Contains(err.Error(), "marshal from Go func()") {
			t.Fatalf("error = %q, want marshal error", err.Error())
		}
	})

	t.Run("key bytes write error", func(t *testing.T) {
		t.Parallel()

		w := &failAfterNWriter{n: 0, err: errors.New("key boom")}
		bw := bufio.NewWriterSize(w, 1)

		// Fill the buffer so writing kb triggers a flush.
		if _, err := bw.WriteString("x"); err != nil {
			t.Fatalf("prefill write error = %v", err)
		}

		first := true
		err := upload.ExportWriteUploadKV(bw, "k", "v", &first)
		if err == nil {
			t.Fatal("WriteUploadKV() error = nil, want non-nil")
		}
		if err.Error() != "key boom" {
			t.Fatalf("error = %q, want %q", err.Error(), "key boom")
		}
	})

	t.Run("colon write error", func(t *testing.T) {
		t.Parallel()

		// `"k"` is 3 bytes. With buffer size 3 it fills the buffer,
		// so writing ":" triggers a flush and fails there.
		w := &failAfterNWriter{n: 0, err: errors.New("colon boom")}
		bw := bufio.NewWriterSize(w, 3)

		first := true
		err := upload.ExportWriteUploadKV(bw, "k", "v", &first)
		if err == nil {
			t.Fatal("WriteUploadKV() error = nil, want non-nil")
		}
		if err.Error() != "colon boom" {
			t.Fatalf("error = %q, want %q", err.Error(), "colon boom")
		}
	})

	t.Run("value bytes write error", func(t *testing.T) {
		t.Parallel()

		// `"k"` + ":" = 4 bytes total buffered before value write.
		// Writing the value then flushes and fails.
		w := &failAfterNWriter{n: 0, err: errors.New("value boom")}
		bw := bufio.NewWriterSize(w, 4)

		first := true
		err := upload.ExportWriteUploadKV(bw, "k", "v", &first)
		if err == nil {
			t.Fatal("WriteUploadKV() error = nil, want non-nil")
		}
		if err.Error() != "value boom" {
			t.Fatalf("error = %q, want %q", err.Error(), "value boom")
		}
	})
}

func TestJoinErr(t *testing.T) {
	t.Parallel()

	first := errors.New("first")
	second := errors.New("second")

	tests := []struct {
		name       string
		err        error
		next       error
		wantFirst  bool
		wantSecond bool
		wantSame   error
	}{
		{
			name:     "both nil",
			wantSame: nil,
		},
		{
			name:      "next nil returns original",
			err:       first,
			wantFirst: true,
			wantSame:  first,
		},
		{
			name:       "original nil returns next",
			next:       second,
			wantSecond: true,
			wantSame:   second,
		},
		{
			name:       "joins both errors",
			err:        first,
			next:       second,
			wantFirst:  true,
			wantSecond: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := upload.ExportJoinErr(tt.err, tt.next)

			if tt.wantSame != nil && got != tt.wantSame {
				t.Fatalf("JoinErr() = %v, want original error %v", got, tt.wantSame)
			}

			if tt.wantSame == nil && tt.err == nil && tt.next == nil && got != nil {
				t.Fatalf("JoinErr() = %v, want nil", got)
			}

			if tt.wantFirst && !errors.Is(got, first) {
				t.Fatalf("JoinErr() = %v, want first error", got)
			}

			if tt.wantSecond && !errors.Is(got, second) {
				t.Fatalf("JoinErr() = %v, want second error", got)
			}
		})
	}
}
