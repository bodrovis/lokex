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
)

var markerFiles = []string{"go.mod", ".git"} // treat either as repo root

// FindProjectRoot returns the nearest dir (starting at startDir) that contains a marker (go.mod/.git).
func FindProjectRoot(startDir string) (string, error) {
	dir := startDir
	for {
		for _, m := range markerFiles {
			if _, err := os.Stat(filepath.Join(dir, m)); err == nil {
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

func projectRoot() (string, error) {
	rootOnce.Do(func() {
		wd, err := os.Getwd()
		if err != nil {
			rootErr = err
			return
		}
		rootDir, rootErr = FindProjectRoot(wd)
	})
	return rootDir, rootErr
}
