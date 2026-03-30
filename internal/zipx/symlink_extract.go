package zipx

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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

	_ = os.Remove(targetAbs)

	if err := validateSymlinkPlacement(f.Name, targetAbs, destReal, linkTarget); err != nil {
		return err
	}

	if err := os.Symlink(linkTarget, targetAbs); err != nil {
		return fmt.Errorf("create symlink: %w", err)
	}

	return nil
}

func readSymlinkTarget(f *zip.File) (string, error) {
	rc, err := f.Open()
	if err != nil {
		return "", err
	}

	const maxLinkTarget = 1 << 20 // 1 MiB safety cap
	linkTargetBytes, rerr := io.ReadAll(io.LimitReader(rc, maxLinkTarget))
	_ = rc.Close()
	if rerr != nil {
		return "", fmt.Errorf("read symlink target: %w", rerr)
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
	parentResolved, err := filepath.EvalSymlinks(filepath.Dir(targetAbs))
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("symlink parent resolve error: %w", err)
		}
		// If parent doesn't exist, mkdirall above does it, so we fallback to intended parent.
		parentResolved = filepath.Dir(targetAbs)
	}

	linkAbs := filepath.Join(parentResolved, filepath.Base(targetAbs))
	if !isPathWithinBase(destReal, linkAbs) {
		return fmt.Errorf("symlink destination escapes extraction root: %q", linkAbs)
	}

	targetCandidate := filepath.Join(parentResolved, linkTarget)
	if !isPathWithinBase(destReal, targetCandidate) {
		return fmt.Errorf("symlink target escapes extraction root: %q -> %q", entryName, linkTarget)
	}

	return nil
}
