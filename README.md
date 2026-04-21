# Lokex

![GitHub Release](https://img.shields.io/github/v/release/bodrovis/lokex)
![CI](https://github.com/bodrovis/lokex/actions/workflows/ci.yml/badge.svg)
[![Code Coverage](https://qlty.sh/gh/bodrovis/projects/lokex/coverage.svg)](https://qlty.sh/gh/bodrovis/projects/lokex)
[![Maintainability](https://qlty.sh/gh/bodrovis/projects/lokex/maintainability.svg)](https://qlty.sh/gh/bodrovis/projects/lokex)

`lokex` is a Go client for uploading and downloading translations from [Lokalise](https://lokalise.com). It provides a thin wrapper around the Lokalise API with retry/backoff, async polling, safe unzipping, and strict upload validation.

> Lokex also has a cross-platform CLI version: [lokex-cli](https://github.com/bodrovis/lokex-cli).

## Installation

```bash
go get github.com/bodrovis/lokex/v2
```

## Usage

### Create a client

```go
import (
    "log"
    "time"

    "github.com/bodrovis/lokex/v2/client"
)

cli, err := client.NewClient("YOUR_API_TOKEN", "LOKALISE_PROJECT_ID", client.WithBackoff(
    1*time.Second,  // min backoff
    5*time.Second,  // max backoff
))
if err != nil {
    log.Fatal(err)
}
```

By default, the base URL is `https://api.lokalise.com/api2/`. You can override it with `client.WithBaseURL("...")` if needed for testing.

### Downloads

Download and unzip a translation bundle into `./locales`:

```go
import (
    "github.com/bodrovis/lokex/v2/client/download"
)

downloader := download.NewDownloader(cli)

ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
defer cancel()

// call DownloadAsync() for the async download flow
url, err := downloader.Download(ctx, "./locales", download.DownloadParams{
    "format": "json",
    // other request params...
})
if err != nil {
    log.Fatal(err)
}

fmt.Println("Bundle downloaded from:", url)
```

Features:

- Retries on rate limiting errors, 5xx, or truncated/corrupted ZIPs.
- Rejects `zip-slip`, symlinks, and oversized bundles.
- Validates content length and zip structure before unzipping.

### Uploads

Upload a JSON file for the English (`en`) locale:

```go
import (
    "github.com/bodrovis/lokex/v2/client/upload"
)

uploader := upload.NewUploader(cli)

fp := filepath.Join(dir, "en.json")

ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
defer cancel()

// srcPath (3rd argument) is optional:
// - if srcPath == "" -> uploader reads from params["filename"]
// - if srcPath != "" -> uploader reads file bytes from srcPath,
//   but still sends params["filename"] to Lokalise API as the remote filename.
pid, err := uploader.Upload(ctx, upload.UploadParams{
	"filename": fp,      // sent to Lokalise (remote filename)
	"lang_iso": "en",
	// other request params...
}, "", true) // srcPath="", poll=true (pass false to skip polling)
if err != nil {
	log.Fatal(err)
}

fmt.Println("Upload finished with process ID:", pid)
```

Features:

- Validates `filename` and ensures it points to a real file (not a directory).
- Auto-encodes file contents to base64 unless `data` is provided.
- Accepts `data` as a pre-encoded string or raw `[]byte`.
- Polls the process until it finishes (unless polling is disabled).

### Batch Uploads

Upload several locale files in one call:

```go
import (
    "context"
    "fmt"
    "log"
    "path/filepath"
    "time"

    "github.com/bodrovis/lokex/v2/client/upload"
)

uploader := upload.NewUploader(cli)

dir := "/path/to/locales"

items := []upload.BatchUploadItem{
    {
        Params: upload.UploadParams{
            "filename": filepath.Join(dir, "en.json"),
            "lang_iso": "en",
        },
        // SrcPath omitted:
        // uploader reads bytes from Params["filename"]
    },
    {
        Params: upload.UploadParams{
            "filename": "locales/%LANG_ISO%.json", // remote filename sent to Lokalise
            "lang_iso": "de",
        },
        SrcPath: filepath.Join(dir, "de.json"), // local file to read bytes from
    },
    {
        Params: upload.UploadParams{
            "filename": "locales/%LANG_ISO%.json",
            "lang_iso": "fr",
        },
        SrcPath: filepath.Join(dir, "fr.json"),
    },
}

ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
defer cancel()

// poll=true:
// - starts all uploads
// - then polls all successfully started processes together
//
// poll=false:
// - returns right after kickoff with per-item process IDs/errors
result, err := uploader.UploadBatch(ctx, items, true)
if err != nil {
    log.Fatal(err) // fatal batch-level error only
}

for _, item := range result.Items {
    if item.Err != nil {
        fmt.Printf("upload failed: index=%d src=%q err=%v\n", item.Index, item.SrcPath, item.Err)
        continue
    }

    fmt.Printf("upload finished: index=%d src=%q process_id=%s\n", item.Index, item.SrcPath, item.ProcessID)
}

if result.HasErrors() {
    fmt.Println("some uploads failed")
}

fmt.Printf("successful process IDs: %#v\n", result.SuccessfulProcessIDs())
```

`UploadBatch` returns:

- `BatchUploadResult` — per-item results, always in the same order as the input slice
- `error` — only for fatal batch-level problems such as a nil uploader/client or an already-cancelled context

Each `BatchUploadResultItem` contains:

- `Index` — original position in the input slice
- `SrcPath` — local source path used for that item
- `ProcessID` — Lokalise process ID for successful kickoff/completion
- `Err` — per-item error; does not fail the whole batch

Notes:

- Upload kickoff runs with a maximum concurrency of 6 files at a time
- Each file still uses the same single-upload logic internally, including retries
- Partial success is supported: one failed file does not discard successful ones
- `SrcPath` is optional per item:
  - if `SrcPath == ""`, uploader reads bytes from `Params["filename"]`
  - if `SrcPath != ""`, uploader reads bytes from `SrcPath`, but still sends `Params["filename"]` to Lokalise as the remote filename

## Testing

Unit tests use [httpmock](https://github.com/jarcoal/httpmock). Integration tests hit the real Lokalise API and require credentials in `.env`.

Run unit tests only:

```bash
go test ./... -v -short
```

Run with full integration tests:

```bash
go test ./... -v
```

## License

(c) [Ilya Krukowski](https://bodrovis.tech). Licensed under BSD 3-Clause
