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
	removePathFn = os.Remove
	symlinkFn    = os.Symlink
	readAllFn    = io.ReadAll
)

func extractSymlinkEntry(f *zip.File, targetAbs, destReal string, p Policy) error {
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

	_ = removePathFn(targetAbs)

	if err := validateSymlinkPlacement(f.Name, targetAbs, destReal, linkTarget); err != nil {
		return err
	}

	if err := symlinkFn(linkTarget, targetAbs); err != nil {
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

	linkTargetBytes, rerr := readAllFn(io.LimitReader(rc, maxLinkTarget+1))
	if rerr != nil {
		return "", fmt.Errorf("read symlink target: %w", rerr)
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
		return fmt.Errorf("absolute symlink target not allowed: %q -> %q", entryName, linkTarget)
	}
	return nil
}

func validateSymlinkPlacement(entryName, targetAbs, destReal, linkTarget string) error {
	parentResolved, err := evalSymlinksPathFn(filepath.Dir(targetAbs))
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("symlink parent resolve error: %w", err)
		}
		// If parent doesn't exist, mkdirall above does it, so we fallback to intended parent.
		parentResolved = filepath.Dir(targetAbs)
	}

	linkAbs := filepath.Join(parentResolved, filepath.Base(targetAbs))
	if !isPathWithinBaseFn(destReal, linkAbs) {
		return fmt.Errorf("symlink destination escapes extraction root: %q", linkAbs)
	}

	targetCandidate := filepath.Join(parentResolved, linkTarget)
	if !isPathWithinBaseFn(destReal, targetCandidate) {
		return fmt.Errorf("symlink target escapes extraction root: %q -> %q", entryName, linkTarget)
	}

	return nil
}
