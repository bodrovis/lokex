package main

import (
	"fmt"
	"os"
	"os/exec"
)

func run(cmd string, args ...string) error {
	c := exec.Command(cmd, args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func ensureInstalled(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}

func main() {
	fmt.Println("Running go vet...")
	_ = run("go", "vet", "./...")

	fmt.Println("Running golangci-lint...")
	_ = run("golangci-lint", "run")

	if ensureInstalled("staticcheck") {
		fmt.Println("Running staticcheck...")
		_ = run("staticcheck", "./...")
	} else {
		fmt.Println("staticcheck not installed (skip)")
	}

	if ensureInstalled("gofumpt") {
		fmt.Println("Running gofumpt...")
		_ = run("gofumpt", "-l", "-w", ".")
	} else {
		fmt.Println("gofumpt not installed (skip)")
	}

	fmt.Println("Done ✔")
}
