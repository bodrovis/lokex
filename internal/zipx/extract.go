package zipx

import (
	"archive/zip"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Unzip extracts srcZip into destDir according to policy p.
// It enforces limits, prevents zip-slip, and skips unsafe entries.
func Unzip(srcZip, destDir string, p Policy) (err error) {
	r, err := zip.OpenReader(srcZip)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := r.Close(); cerr != nil {
			err = errors.Join(err, fmt.Errorf("close zip: %w", cerr))
		}
	}()

	destReal, err := prepareExtractionRoot(destDir)
	if err != nil {
		return err
	}

	if p.MaxFiles > 0 && len(r.File) > p.MaxFiles {
		return fmt.Errorf("zip too many files: %d", len(r.File))
	}

	var totalWritten int64
	for _, f := range r.File {
		n, err := extractEntry(f, destDir, destReal, p)
		if err != nil {
			return err
		}
		totalWritten += n
		if p.MaxTotalBytes > 0 && totalWritten > p.MaxTotalBytes {
			return fmt.Errorf("zip too large uncompressed (actual): %d > %d", totalWritten, p.MaxTotalBytes)
		}
	}

	return nil
}

func prepareExtractionRoot(destDir string) (string, error) {
	if err := os.MkdirAll(destDir, 0o700); err != nil {
		return "", err
	}

	destAbs, err := filepath.Abs(destDir)
	if err != nil {
		return "", err
	}

	destReal := destAbs
	if dr, err := filepath.EvalSymlinks(destAbs); err == nil && dr != "" {
		destReal = dr
	}
	return destReal, nil
}
