# Lokex

`lokex` is a Go client for uploading/downloading Lokalise translations.

## Usage

### Create a client

```go
import (
  "github.com/bodrovis/lokex/client"
)

client, err := client.NewClient("YOUR_API_TOKEN", "LOKALISE_PROJECT_ID", nil)
if err != nil {
    log.Fatal(err)
}
// Or, configure client with helper methods, for example:
// client, err := client.NewClient(token, projectID, client.WithBackoff(
//     1*time.Second,
//     5*time.Second,
// ))
```

### Downloads

Download and unzip the translation bundle into `./locales`:

```go
downloader := client.NewDownloader(client)

ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
defer cancel()

// call DownloadAsync() with the same arguments to perform async download
url, err := downloader.Download(ctx, "./locales", client.DownloadParams{
    "format": "json",
    // Pass other API request params here...
})
if err != nil {
    log.Fatal(err)
}

fmt.Println("Bundle downloaded from:", url)
```

### Uploads

Upload JSON file for English (`en`) locale:

```go
uploader := client.NewUploader(cli)

fp := filepath.Join(dir, "en.json")

ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
defer cancel()

pid, err := uploader.Upload(ctx, client.UploadParams{
    "filename": fp,
    "lang_iso": "en",
    // add other API params ...
})
if err != nil {
    log.Fatal(err)
}

fmt.Println("Upload process ID completed:", pid)
```

## Testing

Run unit tests:

```bash
go test ./... -v -short
```

Run full integration tests (requires valid API token in `.env`):

```bash
go test ./... -v
```

## License

BSD 3 Clause
