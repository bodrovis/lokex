# Lokex

`lokex` is a Go client for uploading/downloading Lokalise translations.

## Usage

### Downloads

Download and unzip the translation bundle into `./locales`:

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

downloader := client.NewDownloader(client)

ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
defer cancel()

url, err := downloader.Download(ctx, "./locales", client.DownloadParams{
    "format": "json",
    // Pass other API request params here...
    // Enable async downloads:
    // "async": true,
})
if err != nil {
    log.Fatal(err)
}

fmt.Println("Bundle downloaded from:", url)
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
