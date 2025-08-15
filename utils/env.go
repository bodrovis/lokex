package utils

import (
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// LoadDotEnv loads variables from a .env file if present.
// Call this early in main() or init() in tests.
func LoadDotEnv(paths ...string) error {
	if len(paths) > 0 {
		return godotenv.Load(paths...)
	}
	// try CWD first
	if err := godotenv.Load(); err == nil {
		return nil
	}

	wd, _ := os.Getwd()
	dir := wd
	for range make([]struct{}, 6) { // up to 6 parent levels
		envPath := filepath.Join(dir, ".env")
		if _, err := os.Stat(envPath); err == nil {
			return godotenv.Load(envPath)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
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
