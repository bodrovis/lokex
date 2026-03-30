package download_test

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/bodrovis/lokex/v2/internal/utils"

	"github.com/jarcoal/httpmock"
)

func buildZip(t *testing.T, entries map[string]string, symlinks map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// regular files
	for name, content := range entries {
		fh := &zip.FileHeader{
			Name:   name,
			Method: zip.Deflate,
		}
		// ensure dirs implied by name are created in archive entries (zip doesn't need explicit dir entries)
		fh.SetMode(0o644)
		w, err := zw.CreateHeader(fh)
		if err != nil {
			t.Fatalf("CreateHeader(%s): %v", name, err)
		}
		if _, err := io.Copy(w, strings.NewReader(content)); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	// symlinks (write target path as file content; unzipSafe should skip or handle)
	for link, target := range symlinks {
		fh := &zip.FileHeader{
			Name:   link,
			Method: zip.Store,
		}
		// mark as symlink
		fh.SetMode(os.ModeSymlink | 0o777)
		w, err := zw.CreateHeader(fh)
		if err != nil {
			t.Fatalf("CreateHeader(symlink %s): %v", link, err)
		}
		if _, err := io.Copy(w, strings.NewReader(target)); err != nil {
			t.Fatalf("write symlink %s: %v", link, err)
		}
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

func registerZipResponder(t *testing.T, url string, zipBytes []byte) {
	t.Helper()
	httpmock.RegisterResponder("GET", url, func(req *http.Request) (*http.Response, error) {
		return httpmock.NewBytesResponse(200, zipBytes), nil
	})
}

func registerZipResponderWithHeaderAsserts(t *testing.T, url string, zipBytes []byte, wantUA string) {
	t.Helper()
	httpmock.RegisterResponder("GET", url, func(req *http.Request) (*http.Response, error) {
		if got := req.Header.Get("User-Agent"); wantUA != "" && got != wantUA {
			t.Fatalf("GET UA = %q, want %q", got, wantUA)
		}
		if got := req.Header.Get("Accept"); got == "" {
			t.Fatalf("GET Accept header missing")
		}
		if got := req.Header.Get("Accept-Encoding"); got != "identity" {
			t.Fatalf("GET Accept-Encoding = %q, want identity", got)
		}
		return httpmock.NewBytesResponse(200, zipBytes), nil
	})
}

func mustJSONBody(t *testing.T, m map[string]any) io.Reader {
	t.Helper()
	r, err := utils.EncodeJSONBody(m)
	if err != nil {
		t.Fatalf("encode body: %v", err)
	}
	return r
}
