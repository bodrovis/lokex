package zipx

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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
) error {
	if !p.AllowSymlinks {
		return nil
	}

	linkTarget, err := readSymlinkTarget(f)
	if err != nil {
		return err
	}

	if err := validateSymlinkTargetString(f.Name, linkTarget); err != nil {
		return err
	}

	if err := validateSymlinkPlacement(f.Name, rel, linkTarget); err != nil {
		return err
	}

	// Remove an existing entry before creating the symlink.
	_ = root.Remove(rel)

	if err := symlinkFn(root, linkTarget, rel); err != nil {
		return fmt.Errorf("create symlink: %w", err)
	}

	return nil
}

func readSymlinkTarget(f *zip.File) (string, error) {
	rc, err := f.Open()
	if err != nil {
		return "", err
	}
	defer func() {
		_ = rc.Close()
	}()

	const maxLinkTarget = 1 << 20 // 1 MiB safety cap

	linkTargetBytes, err := readAllFn(
		io.LimitReader(rc, maxLinkTarget+1),
	)
	if err != nil {
		return "", fmt.Errorf("read symlink target: %w", err)
	}

	if len(linkTargetBytes) > maxLinkTarget {
		return "", fmt.Errorf("symlink target too large")
	}

	return strings.TrimSpace(string(linkTargetBytes)), nil
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
