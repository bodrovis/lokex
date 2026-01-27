# Lokex

![GitHub Release](https://img.shields.io/github/v/release/bodrovis/lokex)
![CI](https://github.com/bodrovis/lokex/actions/workflows/ci.yml/badge.svg)

`lokex` is a Go client for uploading and downloading translations from [Lokalise](https://lokalise.com). It provides a thin wrapper around the Lokalise API with retry/backoff, async polling, safe unzipping, and strict upload validation.

## Installation

```bash
go get github.com/bodrovis/lokex
```

## Usage

### Create a client

```go
import (
    "log"
    "time"

    "github.com/bodrovis/lokex/client"
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
downloader := client.NewDownloader(cli)

ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
defer cancel()

// call DownloadAsync() for the async download flow
url, err := downloader.Download(ctx, "./locales", client.DownloadParams{
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
uploader := client.NewUploader(cli)

fp := filepath.Join(dir, "en.json")

ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
defer cancel()

// srcPath (3rd argument) is optional:
// - if srcPath == "" -> uploader reads from params["filename"]
// - if srcPath != "" -> uploader reads file bytes from srcPath,
//   but still sends params["filename"] to Lokalise API as the remote filename.
pid, err := uploader.Upload(ctx, client.UploadParams{
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
