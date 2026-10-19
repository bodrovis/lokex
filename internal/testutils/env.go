package testutils

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// LoadDotEnv loads variables from a .env file.
// Search order: explicit paths, CWD, project root.
func LoadDotEnv(paths ...string) error {
	if len(paths) > 0 {
		return godotenv.Load(paths...)
	}

	if err := godotenv.Load(); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	root, err := ProjectRoot()
	if err != nil {
		return os.ErrNotExist
	}

	return godotenv.Load(filepath.Join(root, ".env"))
}

// GetEnv returns the environment variable value if non-empty,
// or the default otherwise.
func GetEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
