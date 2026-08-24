package zipx

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

func normalizeZipEntryPath(name string) (string, error) {
	name = strings.ReplaceAll(name, `\`, `/`)

	if strings.IndexByte(name, 0) != -1 {
		return "", fmt.Errorf(
			"invalid file name (NUL) in zip: %q",
			name,
		)
	}

	if strings.HasPrefix(name, "/") {
		return "", fmt.Errorf(
			"unsafe absolute path in zip: %q",
			name,
		)
	}

	rel := path.Clean(name)

	if rel == "." {
		return "", nil
	}

	for seg := range strings.SplitSeq(rel, "/") {
		if seg == ".." {
			return "", fmt.Errorf(
				"unsafe path traversal in zip (.. segment): %q",
				name,
			)
		}
	}

	local, err := filepath.Localize(rel)
	if err != nil {
		return "", fmt.Errorf(
			"invalid zip entry path: %q: %w",
			name,
			err,
		)
	}

	return local, nil
}
