package testutils_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/bodrovis/lokex/v2/internal/testutils"
)

func resetProjectRootCache(t *testing.T) {
	t.Helper()

	testutils.TestOnlySetGetwd(os.Getwd)
	testutils.TestOnlyResetProjectRootCache()

	t.Cleanup(func() {
		testutils.TestOnlySetGetwd(os.Getwd)
		testutils.TestOnlyResetProjectRootCache()
	})
}

func TestFindProjectRoot_ReturnsNearestMarkerDir(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	root := filepath.Join(base, "repo")
	start := filepath.Join(root, "a", "b", "c")

	if err := os.MkdirAll(start, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/test\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(go.mod) error = %v", err)
	}

	got, err := testutils.FindProjectRoot(start)
	if err != nil {
		t.Fatalf("FindProjectRoot() error = %v", err)
	}
	if got != root {
		t.Fatalf("root = %q, want %q", got, root)
	}
}

func TestFindProjectRoot_ReturnsNotExistWhenNoMarkersFound(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	start := filepath.Join(base, "a", "b", "c")

	if err := os.MkdirAll(start, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	got, err := testutils.FindProjectRoot(start)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("error = %v, want %v", err, os.ErrNotExist)
	}
	if got != "" {
		t.Fatalf("root = %q, want empty string", got)
	}
}

func TestProjectRoot_ReturnsGetwdError(t *testing.T) {
	resetProjectRootCache(t)

	wantErr := errors.New("getwd boom")
	testutils.TestOnlySetGetwd(func() (string, error) {
		return "", wantErr
	})

	got, err := testutils.ProjectRoot()
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if got != "" {
		t.Fatalf("root = %q, want empty string", got)
	}
}

func TestProjectRoot_CachesResult(t *testing.T) {
	resetProjectRootCache(t)

	base := t.TempDir()
	root := filepath.Join(base, "repo")
	start := filepath.Join(root, "nested")

	if err := os.MkdirAll(start, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("stub"), 0o644); err != nil {
		t.Fatalf("WriteFile(.git) error = %v", err)
	}

	calls := 0
	testutils.TestOnlySetGetwd(func() (string, error) {
		calls++
		return start, nil
	})

	got1, err1 := testutils.ProjectRoot()
	if err1 != nil {
		t.Fatalf("ProjectRoot() first error = %v", err1)
	}
	got2, err2 := testutils.ProjectRoot()
	if err2 != nil {
		t.Fatalf("ProjectRoot() second error = %v", err2)
	}

	if got1 != root {
		t.Fatalf("first root = %q, want %q", got1, root)
	}
	if got2 != root {
		t.Fatalf("second root = %q, want %q", got2, root)
	}
	if calls != 1 {
		t.Fatalf("getwd calls = %d, want %d", calls, 1)
	}
}
