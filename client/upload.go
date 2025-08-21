package client

type Uploader struct {
	client *Client
}

type UploadParams map[string]any

func NewUploader(c *Client) *Uploader {
	return &Uploader{
		client: c,
	}
}
