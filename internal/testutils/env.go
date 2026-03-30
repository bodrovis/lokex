package testutils

import (
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

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
