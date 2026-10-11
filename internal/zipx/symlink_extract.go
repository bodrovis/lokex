package zipx

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var (
	readAllFn = io.ReadAll

	// Keep this injectable because creating symlinks may be restricted
	// on some platforms/environments, especially Windows.
	symlinkFn = func(root *os.Root, target, name string) error {
		return root.Symlink(target, name)
	}
)

func extractSymlinkEntry(
	f *zip.File,
	root *os.Root,
	rel string,
	p Policy,
	limit byteLimit,
) (int64, error) {
	if !p.AllowSymlinks {
		return 0, nil
	}

	linkTarget, n, err := readSymlinkTarget(f, limit)
	if err != nil {
		return 0, err
	}

	if err := validateSymlinkTargetString(f.Name, linkTarget); err != nil {
		return 0, err
	}

	if err := validateSymlinkPlacement(f.Name, rel, linkTarget); err != nil {
		return 0, err
	}

	if err := symlinkFn(root, linkTarget, rel); err != nil {
		return 0, fmt.Errorf("create symlink: %w", err)
	}

	return n, nil
}

func readSymlinkTarget(
	f *zip.File,
	limit byteLimit,
) (target string, size int64, retErr error) {
	rc, err := f.Open()
	if err != nil {
		return "", 0, err
	}

	defer func() {
		if err := rc.Close(); err != nil {
			retErr = errors.Join(
				retErr,
				fmt.Errorf("close symlink target: %w", err),
			)
		}
	}()

	const maxLinkTarget int64 = 1 << 20

	cap := maxLinkTarget
	if limit.enabled && limit.max < cap {
		cap = limit.max
	}

	data, err := readAllFn(io.LimitReader(rc, cap+1))
	if err != nil {
		return "", 0, fmt.Errorf("read symlink target: %w", err)
	}

	if int64(len(data)) > cap {
		return "", 0, fmt.Errorf("symlink target exceeds size limit")
	}

	return string(data), int64(len(data)), nil
}

func validateSymlinkTargetString(entryName, linkTarget string) error {
	if linkTarget == "" {
		return fmt.Errorf("empty symlink target: %q", entryName)
	}

	if filepath.IsAbs(linkTarget) || filepath.VolumeName(linkTarget) != "" {
		return fmt.Errorf(
			"absolute symlink target not allowed: %q -> %q",
			entryName,
			linkTarget,
		)
	}

	return nil
}

func validateSymlinkPlacement(entryName, rel, linkTarget string) error {
	targetRel := filepath.Join(filepath.Dir(rel), linkTarget)

	if !filepath.IsLocal(targetRel) {
		return fmt.Errorf(
			"symlink target escapes extraction root: %q -> %q",
			entryName,
			linkTarget,
		)
	}

	return nil
}
