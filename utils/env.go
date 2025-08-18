package utils

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/joho/godotenv"
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

// LoadDotEnv loads variables from a .env file.
// Priority: explicit paths -> CWD -> project root (go.mod/.git) -> not found.
func LoadDotEnv(paths ...string) error {
	if len(paths) > 0 {
		return godotenv.Load(paths...)
	}

	// try CWD
	if err := godotenv.Load(); err == nil {
		return nil
	}

	// try project root
	if rd, err := projectRoot(); err == nil {
		if _, err := os.Stat(filepath.Join(rd, ".env")); err == nil {
			return godotenv.Load(filepath.Join(rd, ".env"))
		}
	}

	return os.ErrNotExist
}

// GetEnv returns the environment variable value if set, or the default.
func GetEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
