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
	// Start JSON object.
	if _, err := w.WriteString("{"); err != nil {
		return err
	}

	first := true
	writeComma := func() error {
		if first {
			first = false
			return nil
		}
		_, err := w.WriteString(",")
		return err
	}

	// Write params except "data" (unordered).
	for k, v := range params {
		if k == "data" {
			continue
		}
		if err := writeUploadKV(w, k, v, &first); err != nil {
			return err
		}
	}

	// Now write "data".
	if err := writeComma(); err != nil {
		return err
	}

	if _, err := w.WriteString(`"data":"`); err != nil {
		return err
	}

	if err := writeUploadData(w, cleanPath, spec); err != nil {
		return err
	}

	// Close string + object.
	_, err := w.WriteString(`"}`)
	return err
}

func writeUploadKV(w *bufio.Writer, k string, v any, first *bool) error {
	if !*first {
		if _, err := w.WriteString(","); err != nil {
			return err
		}
	} else {
		*first = false
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
	// Caller provided base64 string -> just write as-is.
	if !spec.useFile && !spec.dataWasBytes {
		_, err := w.WriteString(spec.dataString)
		return err
	}

	// Pick a reader source (file or bytes).
	var (
		r         io.Reader
		closeFile func() error
	)

	switch {
	case spec.useFile:
		f, err := openFile(cleanPath)
		if err != nil {
			return err
		}
		r = f
		closeFile = f.Close

	case spec.dataWasBytes:
		r = bytes.NewReader(spec.dataBytes)
	}

	enc := base64.NewEncoder(base64.StdEncoding, w)

	_, err := io.Copy(enc, r)

	// Close file (if any), but don’t clobber existing error.
	if closeFile != nil {
		if cerr := closeFile(); cerr != nil {
			if err == nil {
				err = cerr
			} else {
				err = errors.Join(err, cerr)
			}
		}
	}

	// Close encoder (flushes final base64 padding).
	if cerr := enc.Close(); cerr != nil {
		if err == nil {
			err = cerr
		} else {
			err = errors.Join(err, cerr)
		}
	}

	return err
}
