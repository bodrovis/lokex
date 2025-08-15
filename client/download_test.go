package client_test

import (
	"context"
	"fmt"
	// "encoding/json"
	// "fmt"
	// "net/http"
	"testing"

	"github.com/bodrovis/lokex/client"
	"github.com/bodrovis/lokex/utils"
	// "github.com/jarcoal/httpmock"
)

var (
	token     string
	projectID string
)

func init() {
	utils.LoadDotEnv()
	token = utils.GetEnv("LOKALISE_API_TOKEN", "secret")
	projectID = utils.GetEnv("LOKALISE_PROJECT_ID", "123.abc")
}

func TestDownloader_Download(t *testing.T) {
	// httpmock.Activate()
	// defer httpmock.DeactivateAndReset()

	// stubBundleURL := "https://cdn.example.com/bundle.zip"

	// target := fmt.Sprintf("https://api.lokalise.com/api2/projects/%s/files/download", projectID)

	// respBody, _ := json.Marshal(map[string]string{
	// 	"bundle_url": stubBundleURL,
	// })

	// httpmock.RegisterResponder("POST", target, func(req *http.Request) (*http.Response, error) {
	// 	if got := req.Header.Get("X-Api-Token"); got != token {
	// 		t.Fatalf("missing/incorrect X-Api-Token: %q", got)
	// 	}
	// 	return httpmock.NewStringResponse(200, string(respBody)), nil
	// })

	downloader := client.NewDownloader(client.NewClient(token, projectID, nil))
	bundleURL, err := downloader.Download(context.Background())
	if err != nil {
		t.Fatalf("Download error: %v", err)
	}
	fmt.Println(bundleURL)
	// if bundleURL != stubBundleURL {
	// 	t.Fatalf("Expected URL %s, got %s", bundleURL, stubBundleURL)
	// }
}
