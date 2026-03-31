package download_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/bodrovis/lokex/v2/client"
	"github.com/bodrovis/lokex/v2/client/download"
)

func TestDoDownloadRequest(t *testing.T) {
	t.Parallel()

	t.Run("bad url returns build request error", func(t *testing.T) {
		t.Parallel()

		d := download.NewDownloader(&client.Client{
			HTTPClient: &http.Client{},
		})

		resp, err := download.ExportDoDownloadRequest(
			d,
			context.Background(),
			&http.Client{},
			"://bad url",
			"",
		)
		if err == nil {
			t.Fatal("DoDownloadRequest() error = nil, want non-nil")
		}
		if !strings.HasPrefix(err.Error(), "build request: ") {
			t.Fatalf("error = %q, want prefix %q", err.Error(), "build request: ")
		}
		if resp != nil {
			t.Fatal("response != nil, want nil on error")
		}
	})
}
