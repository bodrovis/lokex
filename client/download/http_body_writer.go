package download

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// writeHTTPBodyAtomically writes src into a temp file next to destPath and renames it on success.
// If wantLen >= 0, it checks that the copied size matches exactly and returns
// io.ErrUnexpectedEOF on mismatch.
func writeHTTPBodyAtomically(destPath string, src io.Reader, wantLen int64) (err error) {
	dir := filepath.Dir(destPath)
	prefix := filepath.Base(destPath) + ".part-"

	tmp, err := os.CreateTemp(dir, prefix)
	if err != nil {
		return fmt.Errorf("create temp zip: %w", err)
	}
	tmpName := tmp.Name()
	closed := false

	defer func() {
		if !closed {
			_ = tmp.Close()
		}
		if err != nil {
			_ = os.Remove(tmpName)
		}
	}()

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

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close zip: %w", err)
	}
	closed = true

	// On Windows, rename over an existing file can be unreliable; remove first.
	_ = os.Remove(destPath)
	if err := os.Rename(tmpName, destPath); err != nil {
		return fmt.Errorf("finalize zip: %w", err)
	}

	return nil
}
