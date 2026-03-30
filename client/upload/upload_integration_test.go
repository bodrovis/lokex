package upload_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bodrovis/lokex/v2/client"
	"github.com/bodrovis/lokex/v2/client/upload"
)

func TestIntegration_Upload(t *testing.T) {
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
	u := upload.NewUploader(cli)

	// Each test gets its own private directory that Go deletes automatically.
	dir := t.TempDir()

	// Create the file inside the temp directory.
	fp := filepath.Join(dir, "en.json")
	if err := os.WriteFile(fp, []byte(`{"hello":"lokalise"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// filename -> what Lokalise sees; srcPath -> where we read bytes from.
	pid, err := u.Upload(ctx, upload.UploadParams{
		"filename": "locales/%LANG_ISO%.json", // demo filename
		"lang_iso": "en",
	}, fp, true)
	if err != nil {
		t.Fatalf("integration upload failed: %v", err)
	}
	if pid == "" {
		t.Fatalf("expected non-empty process id")
	}
}
