package zipx

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var (
	filepathRelFn      = filepath.Rel
	lstatFn            = os.Lstat
	evalSymlinksPathFn = filepath.EvalSymlinks
	isPathWithinBaseFn = isPathWithinBase
	absFilepath        = filepath.Abs
)

// isPathWithinBase checks if absPath (absolute, resolved) is under baseAbs (absolute, resolved)
func isPathWithinBase(baseAbs, absPath string) bool {
	rel, err := filepathRelFn(baseAbs, absPath)
	if err != nil {
		return false
	}
	relClean := filepath.Clean(rel)
	return relClean != ".." && !strings.HasPrefix(relClean, ".."+string(filepath.Separator))
}

func normalizeZipEntryPath(name string) (string, error) {
	name = strings.ReplaceAll(name, `\`, `/`)

	if strings.IndexByte(name, 0) != -1 {
		return "", fmt.Errorf("invalid file name (NUL) in zip: %q", name)
	}

	rel := path.Clean(name)

	for strings.HasPrefix(rel, "/") || strings.HasPrefix(rel, "./") {
		rel = strings.TrimPrefix(strings.TrimPrefix(rel, "/"), "./")
	}
	if rel == "" || rel == "." {
		return "", nil
	}

	for seg := range strings.SplitSeq(rel, "/") {
		if seg == ".." {
			return "", fmt.Errorf("unsafe path traversal in zip (.. segment): %q", name)
		}
	}

	return rel, nil
}

func resolveTargetPath(destDir, destReal, rel, originalName string) (string, error) {
	cand := filepath.FromSlash(rel)

	if filepath.IsAbs(cand) || filepath.VolumeName(cand) != "" {
		return "", fmt.Errorf("unsafe absolute path in zip: %q", originalName)
	}

	targetPath := filepath.Join(destDir, cand)
	targetAbs, err := absFilepath(targetPath)
	if err != nil {
		return "", err
	}

	if !isPathWithinBaseFn(destReal, targetAbs) {
		return "", fmt.Errorf("unsafe path escape: %q", originalName)
	}

	return targetAbs, nil
}

func pathHasSymlinkOutside(destRoot, file string) (bool, error) {
	rel, err := filepathRelFn(destRoot, file)
	if err != nil {
		return true, err
	}
	cur := destRoot
	for seg := range strings.SplitSeq(rel, string(filepath.Separator)) {
		if seg == "" || seg == "." {
			continue
		}
		cur = filepath.Join(cur, seg)
		fi, err := lstatFn(cur)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return false, err
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			real, err := evalSymlinksPathFn(cur)
			if err != nil {
				return true, err
			}
			if real != destRoot && !strings.HasPrefix(real, destRoot+string(filepath.Separator)) {
				return true, nil
			}
		}
	}
	return false, nil
}
