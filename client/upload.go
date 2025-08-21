package client

import (
	"context"
	"encoding/base64"
	"fmt"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bodrovis/lokex/utils"
)

type Uploader struct {
	client *Client
}

type UploadParams map[string]any

type UploadResponse struct {
	Process struct {
		ProcessID string `json:"process_id"`
	} `json:"process"`
}

func NewUploader(c *Client) *Uploader {
	return &Uploader{
		client: c,
	}
}

func (u *Uploader) Upload(ctx context.Context, params UploadParams) (string, error) {
	// copy to avoid mutating caller's map
	body := make(map[string]any, len(params)+1)
	maps.Copy(body, params)

	raw, ok := body["filename"]
	if !ok {
		return "", fmt.Errorf("upload: missing 'filename' param")
	}
	name, ok := raw.(string)
	if !ok || strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("upload: 'filename' must be a non-empty string")
	}
	cleanPath := filepath.Clean(name)

	// sanity: ensure it's not a directory
	if fi, err := os.Stat(cleanPath); err != nil {
		return "", fmt.Errorf("upload: stat %q: %w", cleanPath, err)
	} else if fi.IsDir() {
		return "", fmt.Errorf("upload: %q is a directory, need a file", cleanPath)
	}

	// Only add "data" if user didn't supply it.
	if _, exists := body["data"]; !exists {
		b, err := os.ReadFile(cleanPath)
		if err != nil {
			return "", fmt.Errorf("upload: read %q: %w", cleanPath, err)
		}
		// strict base64 (StdEncoding already strict, no line breaks)
		body["data"] = base64.StdEncoding.EncodeToString(b)
	} else {
		// Optional: normalize existing "data" to string for JSON encoding
		switch v := body["data"].(type) {
		case []byte:
			body["data"] = base64.StdEncoding.EncodeToString(v)
		case string:
			// assume caller already provided base64
		default:
			return "", fmt.Errorf("upload: 'data' must be string or []byte, got %T", v)
		}
	}

	buf, err := utils.EncodeJSONBody(body)
	if err != nil {
		return "", fmt.Errorf("upload: encode body: %w", err)
	}

	var resp UploadResponse
	path := u.client.projectPath("files/upload")
	if err := u.client.doWithRetry(ctx, http.MethodPost, path, buf, &resp); err != nil {
		return "", fmt.Errorf("upload: %w", err)
	}

	processID := resp.Process.ProcessID
	if strings.TrimSpace(processID) == "" {
		return "", fmt.Errorf("upload: empty process id in response")
	}

	// Poll this single process until it finishes or times out
	results, err := u.client.PollProcesses(ctx, []string{processID})
	if err != nil {
		return "", fmt.Errorf("upload: poll processes: %w", err)
	}
	if len(results) == 0 {
		return "", fmt.Errorf("upload: no process results returned (process_id=%s)", processID)
	}

	completed := results[0]
	if completed.Status == "finished" {
		return processID, nil
	}

	return "", fmt.Errorf("upload: process %s did not finish (status=%s)", completed.ProcessID, completed.Status)
}
