package download

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"os"

	"github.com/bodrovis/lokex/v2/client"
	"github.com/bodrovis/lokex/v2/client/internal/background"
)

type ExportSyncCloseFile interface {
	io.Writer
	Sync() error
	Close() error
	Name() string
}

func ExportWriteHTTPBodyAtomically(destPath string, src io.Reader, wantLen int64) error {
	return writeHTTPBodyAtomically(destPath, src, wantLen)
}

func ExportValidateBundleURL(raw string) (string, error) {
	return validateBundleURL(raw)
}

func ExportIsBlockedIP(ip net.IP) bool {
	return isBlockedIP(ip)
}

func ExportMustCIDR(s string) *net.IPNet {
	return mustCIDR(s)
}

func ExportCopyAndValidate(tmp ExportSyncCloseFile, src io.Reader, wantLen int64) error {
	return copyAndValidate(tmp, src, wantLen)
}

func ExportFinalizeAtomicWrite(tmp ExportSyncCloseFile, tmpName, destPath string, closed *bool) error {
	return finalizeAtomicWrite(tmp, tmpName, destPath, closed)
}

func ExportSetRenameFileForTest(fn func(oldpath, newpath string) error) func() {
	prev := renameFile
	renameFile = fn
	return func() { renameFile = prev }
}

func ExportSetRemoveFileForTest(fn func(name string) error) func() {
	prev := removeFile
	removeFile = fn
	return func() { removeFile = prev }
}

func ExportDownloadOnce(
	d *Downloader,
	ctx context.Context,
	urlStr, destPath, ua string,
) error {
	return d.downloadOnce(ctx, urlStr, destPath, ua)
}

func ExportDownloadOncePrecheck(
	d *Downloader,
	ctx context.Context,
	urlStr, destPath string,
) (*http.Client, string, string, error) {
	return d.downloadOncePrecheck(ctx, urlStr, destPath)
}

func ExportSetDoDownloadRequestForTest(
	fn func(
		d *Downloader,
		ctx context.Context,
		httpc *http.Client,
		urlStr, ua string,
	) (*http.Response, error),
) func() {
	prev := doDownloadRequestFn
	doDownloadRequestFn = fn
	return func() {
		doDownloadRequestFn = prev
	}
}

func ExportDoDownloadRequest(
	d *Downloader,
	ctx context.Context,
	httpc *http.Client,
	urlStr, ua string,
) (*http.Response, error) {
	return d.doDownloadRequest(ctx, httpc, urlStr, ua)
}

func ExportNewDownloaderWithClientForTest(c *client.Client) *Downloader {
	return &Downloader{client: c}
}

func ExportFetchBundleAsyncPrecheck(
	d *Downloader,
	ctx context.Context,
	body io.Reader,
) (context.Context, error) {
	return d.fetchBundleAsyncPrecheck(ctx, body)
}

func ExportStartAsyncDownload(
	d *Downloader,
	ctx context.Context,
	body io.Reader,
) (string, error) {
	return d.startAsyncDownload(ctx, body)
}

func ExportPollAsyncDownloadProcess(
	d *Downloader,
	ctx context.Context,
	pid string,
) (background.QueuedProcess, error) {
	return d.pollAsyncDownloadProcess(ctx, pid)
}

func ExportInterpretAsyncDownloadProcess(p background.QueuedProcess) (string, error) {
	return interpretAsyncDownloadProcess(p)
}

func ExportFinishedAsyncDownloadURL(p background.QueuedProcess) (string, error) {
	return finishedAsyncDownloadURL(p)
}

func ExportDownloadAndUnzipPrecheck(
	d *Downloader,
	ctx context.Context,
	bundleURL, destDir string,
) (context.Context, string, string, error) {
	return d.downloadAndUnzipPrecheck(ctx, bundleURL, destDir)
}

func ExportEnsureDestDir(destDir string) error {
	return ensureDestDir(destDir)
}

func ExportCreateDownloadTempDir() (string, func(), error) {
	return createDownloadTempDir()
}

func ExportNewDownloaderWithClientAndHTTPClientForTest(c *client.Client) *Downloader {
	return &Downloader{client: c}
}

func ExportSetMkdirAllForTest(fn func(path string, perm os.FileMode) error) func() {
	prev := mkdirAll
	mkdirAll = fn
	return func() {
		mkdirAll = prev
	}
}

func ExportSetMkdirTempForTest(fn func(dir, pattern string) (string, error)) func() {
	prev := mkdirTemp
	mkdirTemp = fn
	return func() {
		mkdirTemp = prev
	}
}

func ExportSetRemoveAllForTest(fn func(path string) error) func() {
	prev := removeAll
	removeAll = fn
	return func() {
		removeAll = prev
	}
}

func ExportDoDownload(
	d *Downloader,
	ctx context.Context,
	unzipTo string,
	params DownloadParams,
	fetch FetchFunc,
) (string, error) {
	return d.doDownload(ctx, unzipTo, params, fetch)
}

func ExportSetDownloadAndUnzipForTest(
	fn func(d *Downloader, ctx context.Context, bundleURL, destDir string) error,
) func() {
	prev := downloadAndUnzipFn
	downloadAndUnzipFn = fn
	return func() {
		downloadAndUnzipFn = prev
	}
}

func ExportNopFetchFunc() FetchFunc {
	return func(context.Context, io.Reader) (string, error) {
		return "https://example.com/bundle.zip", nil
	}
}

func ExportSetEncodeJSONBodyForTest(
	fn func(body any) (*bytes.Reader, error),
) func() {
	prev := encodeJSONBody
	encodeJSONBody = fn
	return func() {
		encodeJSONBody = prev
	}
}
