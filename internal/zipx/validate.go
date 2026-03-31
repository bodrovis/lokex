package zipx

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
)

// Validate checks that zipPath is a readable ZIP file.
// Returns io.ErrUnexpectedEOF if it is not.
func Validate(zipPath string) (err error) {
	zr, err := openZipReader(zipPath)
	if err != nil {
		if errors.Is(err, zip.ErrFormat) || errors.Is(err, io.ErrUnexpectedEOF) {
			return fmt.Errorf("zip validate: %w", io.ErrUnexpectedEOF)
		}
		return fmt.Errorf("zip validate open: %w", err)
	}
	defer func() {
		if cerr := zr.Close(); err == nil && cerr != nil {
			err = fmt.Errorf("zip validate close: %w", cerr)
		}
	}()
	return nil
}
