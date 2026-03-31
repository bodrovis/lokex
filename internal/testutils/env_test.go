package testutils_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/bodrovis/lokex/v2/internal/testutils"
)

func TestLoadDotEnv_ExplicitPaths_Success(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env.explicit")
	key := "TESTUTILS_EXPLICIT_PATH_SUCCESS"

	if err := os.WriteFile(envPath, []byte(key+"=from_explicit\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	t.Setenv(key, "")
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("Unsetenv() error = %v", err)
	}

	err := testutils.LoadDotEnv(envPath)
	if err != nil {
		t.Fatalf("LoadDotEnv() error = %v", err)
	}

	if got := os.Getenv(key); got != "from_explicit" {
		t.Fatalf("env %q = %q, want %q", key, got, "from_explicit")
	}
}

func TestLoadDotEnv_ExplicitPaths_Error(t *testing.T) {
	err := testutils.LoadDotEnv("/definitely/not/found/.env")
	if err == nil {
		t.Fatal("LoadDotEnv() error = nil, want error")
	}
}

func TestLoadDotEnv_LoadsFromCWD(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	key := "TESTUTILS_CWD_SUCCESS"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(key+"=from_cwd\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	t.Setenv(key, "")
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("Unsetenv() error = %v", err)
	}

	err := testutils.LoadDotEnv()
	if err != nil {
		t.Fatalf("LoadDotEnv() error = %v", err)
	}

	if got := os.Getenv(key); got != "from_cwd" {
		t.Fatalf("env %q = %q, want %q", key, got, "from_cwd")
	}
}

func TestLoadDotEnv_LoadsFromProjectRootWhenMissingInCWD(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/test\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(go.mod) error = %v", err)
	}

	key := "TESTUTILS_PROJECT_ROOT_SUCCESS"
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte(key+"=from_project_root\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(.env) error = %v", err)
	}

	testutils.TestOnlyResetProjectRootCache()
	testutils.TestOnlySetGetwd(func() (string, error) {
		return nested, nil
	})
	t.Cleanup(func() {
		testutils.TestOnlySetGetwd(os.Getwd)
		testutils.TestOnlyResetProjectRootCache()
	})

	t.Setenv(key, "")
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("Unsetenv() error = %v", err)
	}

	err := testutils.LoadDotEnv()
	if err != nil {
		t.Fatalf("LoadDotEnv() error = %v", err)
	}

	if got := os.Getenv(key); got != "from_project_root" {
		t.Fatalf("env %q = %q, want %q", key, got, "from_project_root")
	}
}

func TestLoadDotEnv_ReturnsNotExistWhenNotFoundAnywhere(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "x", "y", "z")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	t.Chdir(nested)

	err := testutils.LoadDotEnv()
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("error = %v, want %v", err, os.ErrNotExist)
	}
}

func TestLoadDotEnv_ReturnsNotExistWhenProjectRootHasNoEnv(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "subdir")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("stub"), 0o644); err != nil {
		t.Fatalf("WriteFile(.git) error = %v", err)
	}

	t.Chdir(nested)

	err := testutils.LoadDotEnv()
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("error = %v, want %v", err, os.ErrNotExist)
	}
}

func TestGetEnv_ReturnsValueWhenSet(t *testing.T) {
	const key = "TESTUTILS_GETENV_SET"
	t.Setenv(key, "actual")

	got := testutils.GetEnv(key, "fallback")
	if got != "actual" {
		t.Fatalf("GetEnv() = %q, want %q", got, "actual")
	}
}

func TestGetEnv_ReturnsDefaultWhenUnset(t *testing.T) {
	const key = "TESTUTILS_GETENV_UNSET"
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("Unsetenv() error = %v", err)
	}

	got := testutils.GetEnv(key, "fallback")
	if got != "fallback" {
		t.Fatalf("GetEnv() = %q, want %q", got, "fallback")
	}
}
