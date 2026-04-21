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

func TestIntegration_UploadBatch_AllSuccess(t *testing.T) {
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

	dir := t.TempDir()

	enPath := filepath.Join(dir, "en.json")
	dePath := filepath.Join(dir, "de.json")
	frPath := filepath.Join(dir, "fr.json")

	if err := os.WriteFile(enPath, []byte(`{"hello":"lokalise","bye":"goodbye"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dePath, []byte(`{"hello":"lokalise","bye":"tschuss"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(frPath, []byte(`{"hello":"lokalise","bye":"au revoir"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	items := []upload.BatchUploadItem{
		{
			Params: upload.UploadParams{
				"filename": "locales/%LANG_ISO%.json",
				"lang_iso": "en",
			},
			SrcPath: enPath,
		},
		{
			Params: upload.UploadParams{
				"filename": "locales/%LANG_ISO%.json",
				"lang_iso": "de",
			},
			SrcPath: dePath,
		},
		{
			Params: upload.UploadParams{
				"filename": "locales/%LANG_ISO%.json",
				"lang_iso": "fr",
			},
			SrcPath: frPath,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	got, err := u.UploadBatch(ctx, items, true)
	if err != nil {
		t.Fatalf("integration batch upload failed: %v", err)
	}

	if len(got.Items) != 3 {
		t.Fatalf("got.Items len = %d, want 3", len(got.Items))
	}
	if got.HasErrors() {
		t.Fatalf("HasErrors() = true, want false; got = %#v", got.Items)
	}

	for i, item := range got.Items {
		if item.Err != nil {
			t.Fatalf("item[%d].Err = %v, want nil", i, item.Err)
		}
		if item.ProcessID == "" {
			t.Fatalf("item[%d].ProcessID is empty, want non-empty", i)
		}
	}

	ids := got.SuccessfulProcessIDs()
	if len(ids) != 3 {
		t.Fatalf("SuccessfulProcessIDs len = %d, want 3; ids = %#v", len(ids), ids)
	}
	for i, id := range ids {
		if id == "" {
			t.Fatalf("SuccessfulProcessIDs[%d] is empty", i)
		}
	}
}

func TestIntegration_UploadBatch_PartialFailure(t *testing.T) {
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

	dir := t.TempDir()

	enPath := filepath.Join(dir, "en.json")
	dePath := filepath.Join(dir, "de.json")
	missingPath := filepath.Join(dir, "missing.json") // intentionally absent

	if err := os.WriteFile(enPath, []byte(`{"hello":"lokalise"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dePath, []byte(`{"bye":"tschuss"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	items := []upload.BatchUploadItem{
		{
			Params: upload.UploadParams{
				"filename": "locales/%LANG_ISO%.json",
				"lang_iso": "en",
			},
			SrcPath: enPath,
		},
		{
			Params: upload.UploadParams{
				"filename": "locales/%LANG_ISO%.json",
				"lang_iso": "de",
			},
			SrcPath: dePath,
		},
		{
			Params: upload.UploadParams{
				"filename": "locales/%LANG_ISO%.json",
				"lang_iso": "fr",
			},
			SrcPath: missingPath,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	got, err := u.UploadBatch(ctx, items, true)
	if err != nil {
		t.Fatalf("UploadBatch() fatal error = %v, want nil (partial failure should stay in item errors)", err)
	}

	if len(got.Items) != 3 {
		t.Fatalf("got.Items len = %d, want 3", len(got.Items))
	}
	if !got.HasErrors() {
		t.Fatal("HasErrors() = false, want true")
	}

	// First two should succeed.
	for i := 0; i < 2; i++ {
		if got.Items[i].Err != nil {
			t.Fatalf("item[%d].Err = %v, want nil", i, got.Items[i].Err)
		}
		if got.Items[i].ProcessID == "" {
			t.Fatalf("item[%d].ProcessID is empty, want non-empty", i)
		}
	}

	// Third should fail locally before upload kickoff.
	if got.Items[2].Err == nil {
		t.Fatal("item[2].Err = nil, want non-nil")
	}
	if got.Items[2].ProcessID != "" {
		t.Fatalf("item[2].ProcessID = %q, want empty string", got.Items[2].ProcessID)
	}

	ids := got.SuccessfulProcessIDs()
	if len(ids) != 2 {
		t.Fatalf("SuccessfulProcessIDs len = %d, want 2; ids = %#v", len(ids), ids)
	}
	for i, id := range ids {
		if id == "" {
			t.Fatalf("SuccessfulProcessIDs[%d] is empty", i)
		}
	}
}
