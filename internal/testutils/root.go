package testutils

import (
	"os"
	"path/filepath"
	"sync"
)

var (
	rootOnce sync.Once
	rootDir  string
	rootErr  error
	getwd    = os.Getwd
)

var markerFiles = []string{"go.mod", ".git"} // treat either as repo root

// FindProjectRoot returns the nearest dir (starting at startDir) that contains a marker (go.mod/.git).
func FindProjectRoot(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}

	for {
		for _, marker := range markerFiles {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				return dir, nil
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

// ProjectRoot returns the cached project root discovered from the current working directory.
func ProjectRoot() (string, error) {
	rootOnce.Do(func() {
		wd, err := getwd()
		if err != nil {
			rootErr = err
			return
		}
		rootDir, rootErr = FindProjectRoot(wd)
	})
	return rootDir, rootErr
}

func TestOnlySetGetwd(fn func() (string, error)) {
	getwd = fn
}

func TestOnlyResetProjectRootCache() {
	rootOnce = sync.Once{}
	rootDir = ""
	rootErr = nil
}
