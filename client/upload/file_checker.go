package upload

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

var statFile = os.Stat

// ensureFileIsRegular stats the path and rejects directories / missing files.
func ensureFileIsRegular(readPath string) error {
	if strings.TrimSpace(readPath) == "" {
		return errors.New("upload: empty file path")
	}

	fi, err := statFile(readPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("upload: file not found: %q: %w", readPath, err)
		}
		return fmt.Errorf("upload: stat %q: %w", readPath, err)
	}
	if fi.IsDir() {
		return fmt.Errorf("upload: %q is a directory, need a file", readPath)
	}
	if !fi.Mode().IsRegular() {
		return fmt.Errorf("upload: %q is not a regular file", readPath)
	}
	return nil
}
