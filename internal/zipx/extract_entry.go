package zipx

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
)

func extractEntry(
	f *zip.File,
	root *os.Root,
	p Policy,
) (int64, error) {
	rel, mode, skip, err := prepareEntryTarget(f, p)
	if err != nil || skip {
		return 0, err
	}

	if mode.IsDir() {
		return 0, extractDirEntry(f, root, rel, p)
	}

	if err := prepareParentDir(root, rel); err != nil {
		return 0, err
	}

	if isSpecialFileMode(mode) {
		return 0, nil
	}

	if mode&os.ModeSymlink != 0 {
		return 0, extractSymlinkEntry(f, root, rel, p)
	}

	return extractRegularFileEntry(f, root, rel, p)
}

func prepareEntryTarget(
	f *zip.File,
	p Policy,
) (rel string, mode os.FileMode, skip bool, err error) {
	rel, err = normalizeZipEntryPath(f.Name)
	if err != nil {
		return "", 0, false, err
	}

	if rel == "" {
		return "", 0, true, nil
	}

	if p.MaxFileBytes > 0 && int64(f.UncompressedSize64) > p.MaxFileBytes {
		return "", 0, false, fmt.Errorf(
			"zip entry too big by header: %s (%d bytes)",
			f.Name,
			f.UncompressedSize64,
		)
	}

	return rel, f.Mode(), false, nil
}

func extractDirEntry(
	f *zip.File,
	root *os.Root,
	rel string,
	p Policy,
) error {
	if err := root.MkdirAll(rel, 0o755); err != nil {
		return err
	}

	if p.PreserveTimes && !f.Modified.IsZero() {
		_ = root.Chtimes(rel, f.Modified, f.Modified)
	}

	return nil
}

func prepareParentDir(root *os.Root, rel string) error {
	parent := filepath.Dir(rel)

	if parent == "." {
		return nil
	}

	return root.MkdirAll(parent, 0o755)
}

func isSpecialFileMode(mode os.FileMode) bool {
	return mode&(os.ModeDevice|os.ModeNamedPipe|os.ModeSocket) != 0
}
