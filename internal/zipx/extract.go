package zipx

import (
	"archive/zip"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type zipReader interface {
	Close() error
	Files() []*zip.File
}

type stdZipReader struct {
	*zip.ReadCloser
}

func (r stdZipReader) Files() []*zip.File {
	return r.File
}

var (
	openZipReader = func(path string) (zipReader, error) {
		r, err := zip.OpenReader(path)
		if err != nil {
			return nil, err
		}
		return stdZipReader{r}, nil
	}

	mkdirAll     = os.MkdirAll
	absPath      = filepath.Abs
	evalSymlinks = filepath.EvalSymlinks
)

// Unzip extracts srcZip into destDir according to policy p.
// It enforces limits, prevents zip-slip, and skips unsafe entries.
func Unzip(srcZip, destDir string, p Policy) (err error) {
	r, err := openZipReader(srcZip)
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

	files := r.Files()
	if p.MaxFiles > 0 && len(files) > p.MaxFiles {
		return fmt.Errorf("zip too many files: %d", len(files))
	}

	var totalWritten int64
	for _, f := range files {
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
	if err := mkdirAll(destDir, 0o700); err != nil {
		return "", err
	}

	destAbs, err := absPath(destDir)
	if err != nil {
		return "", err
	}

	destReal := destAbs
	if dr, err := evalSymlinks(destAbs); err == nil && dr != "" {
		destReal = dr
	}
	return destReal, nil
}
