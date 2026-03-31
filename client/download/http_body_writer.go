package download

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var (
	createTempFile = os.CreateTemp
	renameFile     = os.Rename
	removeFile     = os.Remove
)

type syncCloseFile interface {
	io.Writer
	Sync() error
	Close() error
	Name() string
}

// writeHTTPBodyAtomically writes src into a temp file next to destPath and renames it on success.
// If wantLen >= 0, it checks that the copied size matches exactly and returns
// io.ErrUnexpectedEOF on mismatch.
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

	tmp, err := createTempFile(dir, prefix)
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
		_ = removeFile(tmpName)
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
	_ = removeFile(destPath)

	if err := renameFile(tmpName, destPath); err != nil {
		return fmt.Errorf("finalize zip: %w", err)
	}

	return nil
}
