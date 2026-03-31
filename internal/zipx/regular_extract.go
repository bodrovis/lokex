package zipx

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"time"
)

var (
	createTempFile = os.CreateTemp
	removeFile     = os.Remove
	renameFile     = os.Rename
	chtimesFile    = os.Chtimes
)

func extractRegularFileEntry(f *zip.File, targetAbs string, p Policy) (int64, error) {
	rc, err := f.Open()
	if err != nil {
		return 0, err
	}

	perm := filePermOrDefault(f.FileInfo().Mode())

	tmpf, tmp, err := createTempOutputFile(targetAbs, perm)
	if err != nil {
		_ = rc.Close()
		return 0, err
	}

	n, werr := copyCapped(tmpf, rc, p.MaxFileBytes)
	werr = closeWithPrecedence(werr, tmpf, rc)
	if werr != nil {
		_ = removeFile(tmp)
		return 0, werr
	}

	if err := finalizeExtractedFile(tmp, targetAbs, f.Modified, p.PreserveTimes); err != nil {
		return 0, err
	}

	return n, nil
}

func filePermOrDefault(mode os.FileMode) os.FileMode {
	perm := mode.Perm()
	if perm == 0 {
		return 0o644
	}
	return perm
}

func createTempOutputFile(targetAbs string, perm os.FileMode) (*os.File, string, error) {
	tmpf, err := createTempFile(filepath.Dir(targetAbs), filepath.Base(targetAbs)+".partial-*")
	if err != nil {
		return nil, "", err
	}

	tmp := tmpf.Name()

	// Best-effort set permissions on the temp file.
	_ = tmpf.Chmod(perm)

	return tmpf, tmp, nil
}

func closeWithPrecedence(current error, closers ...io.Closer) error {
	err := current
	for _, c := range closers {
		if c == nil {
			continue
		}
		if cerr := c.Close(); err == nil && cerr != nil {
			err = cerr
		}
	}
	return err
}

func finalizeExtractedFile(tmp, targetAbs string, modified time.Time, preserveTimes bool) error {
	// On Windows, rename over existing file may fail. Remove first.
	_ = removeFile(targetAbs)
	if err := renameFile(tmp, targetAbs); err != nil {
		_ = removeFile(tmp)
		return err
	}

	if preserveTimes && !modified.IsZero() {
		_ = chtimesFile(targetAbs, modified, modified)
	}

	return nil
}
