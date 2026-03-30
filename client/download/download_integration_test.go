package download_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bodrovis/lokex/v2/client"
	"github.com/bodrovis/lokex/v2/client/download"
)

func TestIntegration_Download(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in -short mode")
	}
	if token == "secret" || projectID == "123.abc" {
		t.Skip("no real Lokalise credentials; skipping integration test")
	}

	cli, err := client.NewClient(token, projectID, nil)
	if err != nil {
		t.Fatal(err)
	}
	dl := download.NewDownloader(cli)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Per-test temp dir; Go removes it automatically.
	destRoot := t.TempDir()
	localesDir := filepath.Join(destRoot, "locales")

	url, err := dl.Download(ctx, localesDir, download.DownloadParams{
		"format": "json",
	})
	if err != nil {
		t.Fatalf("integration request failed: %v", err)
	}
	if !strings.HasPrefix(url, "https://") {
		t.Fatalf("unexpected bundle URL: %q", url)
	}

	entries, err := os.ReadDir(localesDir)
	if err != nil {
		t.Fatalf("cannot read locales dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("locales dir is empty")
	}

	foundFile := false
	err = filepath.WalkDir(localesDir, func(path string, de os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !de.IsDir() {
			foundFile = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk locales dir: %v", err)
	}
	if !foundFile {
		t.Fatalf("no files found under locales dir (only directories present)")
	}
}

func TestIntegration_DownloadAsync(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in -short mode")
	}
	if token == "secret" || projectID == "123.abc" {
		t.Skip("no real Lokalise credentials; skipping integration test")
	}

	cli, err := client.NewClient(token, projectID, nil)
	if err != nil {
		t.Fatal(err)
	}
	dl := download.NewDownloader(cli)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()

	// Per-test temp dir; Go removes it automatically.
	destRoot := t.TempDir()
	localesDir := filepath.Join(destRoot, "locales-async")

	url, err := dl.DownloadAsync(ctx, localesDir, download.DownloadParams{
		"format": "json",
	})
	if err != nil {
		t.Fatalf("integration request failed: %v", err)
	}
	if !strings.HasPrefix(url, "https://") {
		t.Fatalf("unexpected bundle URL: %q", url)
	}

	entries, err := os.ReadDir(localesDir)
	if err != nil {
		t.Fatalf("cannot read locales dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("locales dir is empty")
	}

	foundFile := false
	err = filepath.WalkDir(localesDir, func(path string, de os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !de.IsDir() {
			foundFile = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk locales dir: %v", err)
	}
	if !foundFile {
		t.Fatalf("no files found under locales dir (only directories present)")
	}
}
