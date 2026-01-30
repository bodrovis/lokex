package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func run(dir, cmd string, args ...string) error {
	c := exec.Command(cmd, args...)
	c.Dir = dir
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func installed(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}

func repoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// We are expected to run from within tools module (tools/ or tools/lint/),
	// so repo root is one level up from tools/.
	// Make it robust: if current dir ends with /tools or /tools/lint, go up accordingly.
	d := wd
	for range [4]struct{}{} {
		base := filepath.Base(d)
		if base == "tools" {
			return filepath.Dir(d), nil
		}
		d = filepath.Dir(d)
	}

	return "", fmt.Errorf("cannot locate repo root: expected to be run under tools/")
}

func main() {
	root, err := repoRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	// Run checks against the ROOT module.
	fmt.Println("===> go vet (root)")
	if err := run(root, "go", "vet", "./..."); err != nil {
		os.Exit(1)
	}

	fmt.Println("===> golangci-lint (root)")
	if err := run(root, "golangci-lint", "run"); err != nil {
		os.Exit(1)
	}

	if installed("staticcheck") {
		fmt.Println("===> staticcheck (root)")
		if err := run(root, "staticcheck", "./..."); err != nil {
			os.Exit(1)
		}
	} else {
		fmt.Println("===> staticcheck not installed (skip)")
	}

	if installed("gofumpt") {
		fmt.Println("===> gofumpt (root)")
		if err := run(root, "gofumpt", "-l", "-w", "."); err != nil {
			os.Exit(1)
		}
	} else {
		fmt.Println("===> gofumpt not installed (skip)")
	}

	fmt.Println("Done ✔")
}
