package upload

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
)

var openFile = func(name string) (io.ReadCloser, error) {
	return os.Open(name)
}

func writeUploadJSON(w *bufio.Writer, params UploadParams, cleanPath string, spec uploadDataSpec) error {
	// Manually build JSON to avoid buffering the whole payload in memory.
	if _, err := w.WriteString("{"); err != nil {
		return err
	}

	first := true
	if err := writeUploadParams(w, params, &first); err != nil {
		return err
	}
	if err := writeUploadDataField(w, cleanPath, spec, &first); err != nil {
		return err
	}

	_, err := w.WriteString("}")
	return err
}

func writeUploadParams(w *bufio.Writer, params UploadParams, first *bool) error {
	// Write all params except "data" (handled separately for streaming).
	for k, v := range params {
		if k == "data" {
			continue
		}
		if err := writeUploadKV(w, k, v, first); err != nil {
			return err
		}
	}
	return nil
}

func writeUploadDataField(w *bufio.Writer, cleanPath string, spec uploadDataSpec, first *bool) error {
	// "data" must be written last and streamed (can be large).
	if err := writeUploadComma(w, first); err != nil {
		return err
	}
	if _, err := w.WriteString(`"data":"`); err != nil {
		return err
	}
	if err := writeUploadData(w, cleanPath, spec); err != nil {
		return err
	}
	_, err := w.WriteString(`"`)
	return err
}

func writeUploadComma(w *bufio.Writer, first *bool) error {
	// Track whether a comma is needed between JSON fields.
	if *first {
		*first = false
		return nil
	}
	_, err := w.WriteString(",")
	return err
}

func writeUploadKV(w *bufio.Writer, k string, v any, first *bool) error {
	// Write "key":value pair with proper comma handling.
	if err := writeUploadComma(w, first); err != nil {
		return err
	}

	// json.Marshal(string) cannot fail
	kb, _ := json.Marshal(k)

	vb, err := json.Marshal(v)
	if err != nil {
		return err
	}

	if _, err := w.Write(kb); err != nil {
		return err
	}
	if _, err := w.WriteString(":"); err != nil {
		return err
	}
	_, err = w.Write(vb)
	return err
}

func writeUploadData(w *bufio.Writer, cleanPath string, spec uploadDataSpec) error {
	// If caller already provided base64 string, write it directly.
	if !spec.useFile && !spec.dataWasBytes {
		_, err := w.WriteString(spec.dataString)
		return err
	}

	// Otherwise stream data (file or bytes) through base64 encoder.
	r, closeFn, err := uploadDataReader(cleanPath, spec)
	if err != nil {
		return err
	}

	enc := base64.NewEncoder(base64.StdEncoding, w)

	_, err = io.Copy(enc, r)
	if closeFn != nil {
		err = joinErr(err, closeFn())
	}
	err = joinErr(err, enc.Close())

	return err
}

func uploadDataReader(cleanPath string, spec uploadDataSpec) (io.Reader, func() error, error) {
	// Select source for upload data.
	switch {
	case spec.useFile:
		f, err := openFile(cleanPath)
		if err != nil {
			return nil, nil, err
		}
		return f, f.Close, nil
	case spec.dataWasBytes:
		return bytes.NewReader(spec.dataBytes), nil, nil
	default:
		return nil, nil, nil
	}
}

func joinErr(err error, next error) error {
	// Combine errors without losing the original one.
	if next == nil {
		return err
	}
	if err == nil {
		return next
	}
	return errors.Join(err, next)
}
