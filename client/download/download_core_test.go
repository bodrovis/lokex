package download_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bodrovis/lokex/v2/client"
	"github.com/bodrovis/lokex/v2/client/download"
	"github.com/bodrovis/lokex/v2/internal/testutils"

	"github.com/jarcoal/httpmock"
)

var (
	token     string
	projectID string
)

func init() {
	if err := testutils.LoadDotEnv(); err != nil {
		log.Printf("warning: could not load .env: %v", err)
	}
	token = testutils.GetEnv("LOKALISE_API_TOKEN", "secret")
	projectID = testutils.GetEnv("LOKALISE_PROJECT_ID", "123.abc")
}

func TestDownloader_Download_SyncFlow(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	postURL := fmt.Sprintf("https://api.lokalise.com/api2/projects/%s/files/download", projectID)
	cdnURL := "https://cdn.example.com/sync.zip"

	// POST → bundle_url
	httpmock.RegisterResponder("POST", postURL, func(req *http.Request) (*http.Response, error) {
		var got map[string]any
		_ = json.NewDecoder(req.Body).Decode(&got)
		if got["format"] != "json" {
			t.Fatalf("format = %v, want json", got["format"])
		}
		return httpmock.NewStringResponse(200, `{"bundle_url":"`+cdnURL+`"}`), nil
	})

	// GET ZIP
	zb := buildZip(t, map[string]string{"ok.txt": "ok"}, nil)
	registerZipResponder(t, cdnURL, zb)

	cli, _ := client.NewClient(token, projectID, nil)
	dl := download.NewDownloader(cli)

	dest := t.TempDir()
	url, err := dl.Download(context.Background(), dest, download.DownloadParams{"format": "json"})
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if url != cdnURL {
		t.Fatalf("bundle url mismatch: %q", url)
	}

	b, err := os.ReadFile(filepath.Join(dest, "ok.txt"))
	if err != nil || string(b) != "ok" {
		t.Fatalf("unzipped wrong: %v %q", err, string(b))
	}
}

func TestDownloader_Download_EmptyUnzipTo(t *testing.T) {
	cli, _ := client.NewClient(token, projectID, nil)
	d := download.NewDownloader(cli)

	_, err := d.Download(context.Background(), "   ", download.DownloadParams{"format": "json"})
	if err == nil || !strings.Contains(err.Error(), "unzipTo is empty") {
		t.Fatalf("want unzipTo is empty error, got %v", err)
	}
}

func TestDownloader_DownloadAsync_FullPipeline(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	// 1) kickoff async
	targetPost := fmt.Sprintf("https://api.lokalise.com/api2/projects/%s/files/async-download", projectID)
	httpmock.RegisterResponder("POST", targetPost,
		httpmock.NewStringResponder(200, `{"process_id":"p1"}`),
	)

	// 2) poll process -> finished + download_url
	targetGet := fmt.Sprintf("https://api.lokalise.com/api2/projects/%s/processes/p1", projectID)
	downloadURL := "https://cdn.example.com/async-full.zip"
	httpmock.RegisterResponder("GET", targetGet, httpmock.NewStringResponder(200, fmt.Sprintf(`{
		"process": {
			"process_id":"p1",
			"status":"finished",
			"details": {"download_url":"%s"}
		}
	}`, downloadURL)))

	// 3) GET zip
	zb := buildZip(t, map[string]string{
		"locales/en/app.json": `{"hello":"async"}`,
		"root.txt":            "ok",
	}, nil)
	registerZipResponder(t, downloadURL, zb)

	cli, _ := client.NewClient(token, projectID, nil)
	d := download.NewDownloader(cli)

	dest := t.TempDir()
	gotURL, err := d.DownloadAsync(context.Background(), dest, download.DownloadParams{"format": "json"})
	if err != nil {
		t.Fatalf("DownloadAsync: %v", err)
	}
	if gotURL != downloadURL {
		t.Fatalf("url=%q, want %q", gotURL, downloadURL)
	}

	// check unzip happened
	b, err := os.ReadFile(filepath.Join(dest, filepath.FromSlash("locales/en/app.json")))
	if err != nil || string(b) != `{"hello":"async"}` {
		t.Fatalf("unzipped file wrong: %v %q", err, string(b))
	}
	b, err = os.ReadFile(filepath.Join(dest, "root.txt"))
	if err != nil || string(b) != "ok" {
		t.Fatalf("unzipped root wrong: %v %q", err, string(b))
	}
}

func TestDownloader_DownloadAsync_EmptyUnzipTo(t *testing.T) {
	cli, _ := client.NewClient(token, projectID, nil)
	d := download.NewDownloader(cli)

	_, err := d.DownloadAsync(context.Background(), "   ", download.DownloadParams{"format": "json"})
	if err == nil || !strings.Contains(err.Error(), "unzipTo is empty") {
		t.Fatalf("want unzipTo is empty error, got %v", err)
	}
}
