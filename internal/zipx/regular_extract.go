package zipx

import (
	"archive/zip"
	"crypto/rand"
	"io"
	"os"
	"path/filepath"
	"time"
)

func extractRegularFileEntry(
	f *zip.File,
	root *os.Root,
	rel string,
	p Policy,
) (int64, error) {
	rc, err := f.Open()
	if err != nil {
		return 0, err
	}

	perm := filePermOrDefault(f.Mode())

	tmpf, tmp, err := createTempOutputFile(root, rel, perm)
	if err != nil {
		_ = rc.Close()
		return 0, err
	}

	n, werr := copyCapped(tmpf, rc, p.MaxFileBytes)
	werr = closeWithPrecedence(werr, tmpf, rc)

	if werr != nil {
		_ = root.Remove(tmp)
		return 0, werr
	}

	if err := finalizeExtractedFile(
		root,
		tmp,
		rel,
		f.Modified,
		p.PreserveTimes,
	); err != nil {
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

func createTempOutputFile(
	root *os.Root,
	rel string,
	perm os.FileMode,
) (*os.File, string, error) {
	tmp := filepath.Join(
		filepath.Dir(rel),
		filepath.Base(rel)+".partial-"+rand.Text(),
	)

	tmpf, err := root.OpenFile(
		tmp,
		os.O_RDWR|os.O_CREATE|os.O_EXCL,
		0o600,
	)
	if err != nil {
		return nil, "", err
	}

	// Match the previous CreateTemp + Chmod behavior.
	_ = tmpf.Chmod(perm)

	return tmpf, tmp, nil
}

func closeWithPrecedence(
	current error,
	closers ...io.Closer,
) error {
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

func finalizeExtractedFile(
	root *os.Root,
	tmp string,
	rel string,
	modified time.Time,
	preserveTimes bool,
) error {
	if err := root.Rename(tmp, rel); err != nil {
		_ = root.Remove(tmp)
		return err
	}

	if preserveTimes && !modified.IsZero() {
		_ = root.Chtimes(rel, modified, modified)
	}

	return nil
}
