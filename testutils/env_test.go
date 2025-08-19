package testutils_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bodrovis/lokex/testutils"
)

func TestFindProjectRoot_UsesGoMod(t *testing.T) {
	root := t.TempDir()
	// make a fake go.mod as the root marker
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/tmp\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	// nest a few dirs
	nested := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdirs: %v", err)
	}

	got, err := testutils.FindProjectRoot(nested)
	if err != nil {
		t.Fatalf("FindProjectRoot: %v", err)
	}
	if got != root {
		t.Fatalf("root = %s; want %s", got, root)
	}
}

func TestLoadDotEnv_ExplicitPath(t *testing.T) {
	tmp := t.TempDir()
	key := "UTILS_TEST_EXPLICIT"
	val := "yup"
	p := writeDotEnv(t, tmp, map[string]string{key: val})

	// ensure not set
	if os.Getenv(key) != "" {
		t.Fatalf("%s unexpectedly set", key)
	}

	if err := testutils.LoadDotEnv(p); err != nil {
		t.Fatalf("LoadDotEnv(explicit): %v", err)
	}
	if got := os.Getenv(key); got != val {
		t.Fatalf("got %q; want %q", got, val)
	}
}

func TestLoadDotEnv_FromCWD(t *testing.T) {
	tmp := t.TempDir()
	key := "UTILS_TEST_CWD"
	val := "here"
	writeDotEnv(t, tmp, map[string]string{key: val})
	chdir(t, tmp)

	if err := testutils.LoadDotEnv(); err != nil {
		t.Fatalf("LoadDotEnv(CWD): %v", err)
	}
	if got := os.Getenv(key); got != val {
		t.Fatalf("got %q; want %q", got, val)
	}
}

func TestLoadDotEnv_FromProjectRoot(t *testing.T) {
	// craft a fake repo root with go.mod and .env
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/tmp\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	key := "UTILS_TEST_ROOT"
	val := "found-at-root"
	writeDotEnv(t, root, map[string]string{key: val})

	// deep nested dir WITHOUT a .env so CWD path fails
	nested := filepath.Join(root, "x", "y", "z")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdirs: %v", err)
	}
	chdir(t, nested)

	if err := testutils.LoadDotEnv(); err != nil {
		t.Fatalf("LoadDotEnv(project root): %v", err)
	}
	if got := os.Getenv(key); got != val {
		t.Fatalf("got %q; want %q", got, val)
	}
}

func TestLoadDotEnv_DoesNotOverrideExisting(t *testing.T) {
	tmp := t.TempDir()
	key := "UTILS_TEST_NO_OVERRIDE"
	writeDotEnv(t, tmp, map[string]string{key: "fromfile"})
	chdir(t, tmp)

	// pre-set should win; godotenv.Load doesn't override by default
	t.Setenv(key, "preset")

	if err := testutils.LoadDotEnv(); err != nil {
		t.Fatalf("LoadDotEnv: %v", err)
	}
	if got := os.Getenv(key); got != "preset" {
		t.Fatalf("expected pre-set env to win, got %q", got)
	}
}

func TestGetEnv(t *testing.T) {
	key := "UTILS_TEST_GETENV"
	def := "default"
	want := "set"

	// when not set -> default
	if got := testutils.GetEnv(key, def); got != def {
		t.Fatalf("GetEnv when unset: got %q; want %q", got, def)
	}

	// when set -> returns set value
	t.Setenv(key, want)
	if got := testutils.GetEnv(key, def); got != want {
		t.Fatalf("GetEnv when set: got %q; want %q", got, want)
	}
}

// --- helpers ---

func writeDotEnv(t *testing.T, dir string, kv map[string]string) string {
	t.Helper()
	var b strings.Builder
	for k, v := range kv {
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(v)
		b.WriteString("\n")
	}
	p := filepath.Join(dir, ".env")
	if err := os.WriteFile(p, []byte(b.String()), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	return p
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
}
