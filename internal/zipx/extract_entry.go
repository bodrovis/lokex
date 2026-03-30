package zipx

import (
	"archive/zip"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func extractEntry(f *zip.File, destDir, destReal string, p Policy) (int64, error) {
	targetAbs, info, mode, skip, err := prepareEntryTarget(f, destDir, destReal, p)
	if err != nil || skip {
		return 0, err
	}

	if info.IsDir() {
		return 0, extractDirEntry(f, targetAbs, p)
	}

	if err := prepareParentDir(targetAbs); err != nil {
		return 0, err
	}

	if err := checkParentSymlinks(destReal, targetAbs, f.Name); err != nil {
		return 0, err
	}

	if isSpecialFileMode(mode) {
		return 0, nil
	}

	if mode&os.ModeSymlink != 0 {
		return 0, extractSymlinkEntry(f, targetAbs, destReal, p)
	}

	return extractRegularFileEntry(f, targetAbs, p)
}

func prepareEntryTarget(f *zip.File, destDir, destReal string, p Policy) (targetAbs string, info fs.FileInfo, mode os.FileMode, skip bool, err error) {
	rel, err := normalizeZipEntryPath(f.Name)
	if err != nil {
		return "", nil, 0, false, err
	}
	if rel == "" {
		return "", nil, 0, true, nil
	}

	if p.MaxFileBytes > 0 && int64(f.UncompressedSize64) > p.MaxFileBytes {
		return "", nil, 0, false, fmt.Errorf("zip entry too big by header: %s (%d bytes)", f.Name, f.UncompressedSize64)
	}

	targetAbs, err = resolveTargetPath(destDir, destReal, rel, f.Name)
	if err != nil {
		return "", nil, 0, false, err
	}

	info = f.FileInfo()
	mode = info.Mode()

	return targetAbs, info, mode, false, nil
}

func extractDirEntry(f *zip.File, targetAbs string, p Policy) error {
	if err := os.MkdirAll(targetAbs, 0o755); err != nil {
		return err
	}
	if p.PreserveTimes && !f.Modified.IsZero() {
		_ = os.Chtimes(targetAbs, f.Modified, f.Modified)
	}
	return nil
}

func prepareParentDir(targetAbs string) error {
	return os.MkdirAll(filepath.Dir(targetAbs), 0o755)
}

func checkParentSymlinks(destReal, targetAbs, entryName string) error {
	if bad, derr := pathHasSymlinkOutside(destReal, targetAbs); derr == nil && bad {
		return fmt.Errorf("unsafe symlink in parents for: %q", entryName)
	} else if derr != nil && !os.IsNotExist(derr) {
		return derr
	}
	return nil
}

func isSpecialFileMode(mode os.FileMode) bool {
	return mode&(os.ModeDevice|os.ModeNamedPipe|os.ModeSocket) != 0
}
