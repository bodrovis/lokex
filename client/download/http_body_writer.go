package download

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var renameFile = os.Rename

type syncCloseFile interface {
	io.Writer
	Sync() error
	Close() error
}

// writeHTTPBodyAtomically writes src to a temporary file next to destPath
// and moves it into place only after the complete body has been written,
// validated, synced, and closed.
//
// This prevents partial downloads from being left at destPath.
// Replacement of an existing destination is not guaranteed to be atomic
// on all platforms.
func writeHTTPBodyAtomically(destPath string, src io.Reader, wantLen int64) (err error) {
	tmp, err := createTempFileNear(destPath)
	if err != nil {
		return err
	}

	tmpName := tmp.Name()
	closed := false

	defer cleanupTempFile(tmp, tmpName, &closed, &err)

	if err := copyAndValidate(tmp, src, wantLen); err != nil {
		return err
	}

	if err := finalizeAtomicWrite(tmp, tmpName, destPath, &closed); err != nil {
		return err
	}

	return nil
}

func createTempFileNear(destPath string) (*os.File, error) {
	dir := filepath.Dir(destPath)
	prefix := filepath.Base(destPath) + ".part-"

	tmp, err := os.CreateTemp(dir, prefix)
	if err != nil {
		return nil, fmt.Errorf("create temp zip: %w", err)
	}
	return tmp, nil
}

func cleanupTempFile(tmp *os.File, tmpName string, closed *bool, retErr *error) {
	if !*closed {
		_ = tmp.Close()
	}
	if *retErr != nil {
		_ = os.Remove(tmpName)
	}
}

func copyAndValidate(tmp syncCloseFile, src io.Reader, wantLen int64) error {
	n, err := io.Copy(tmp, src)
	if err != nil {
		return fmt.Errorf("write zip: %w", err)
	}

	if wantLen >= 0 && n != wantLen {
		return fmt.Errorf("incomplete download: got %d of %d: %w", n, wantLen, io.ErrUnexpectedEOF)
	}

	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync zip: %w", err)
	}

	return nil
}

func finalizeAtomicWrite(tmp syncCloseFile, tmpName, destPath string, closed *bool) error {
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close zip: %w", err)
	}
	*closed = true

	// On Windows, rename over an existing file can be unreliable; remove first.
	// Ignore remove error: destination may not exist yet.
	_ = os.Remove(destPath)

	if err := renameFile(tmpName, destPath); err != nil {
		return fmt.Errorf("finalize zip: %w", err)
	}

	return nil
}
