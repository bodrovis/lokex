package upload

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/bodrovis/lokex/v2/internal/utils"
)

// UploadResponse mirrors the minimal shape we expect from /files/upload.
type UploadResponse struct {
	Process struct {
		ProcessID string `json:"process_id"`
	} `json:"process"`
}

// Read exists only so uploadBodyFactory satisfies io.Reader.
// Retry-aware request execution uses NewBody when available and should not
// consume this reader directly.
func (f uploadBodyFactory) Read(p []byte) (int, error) {
	return 0, io.EOF
}

func (u *Uploader) kickoffUploadStreaming(ctx context.Context, body UploadParams, cleanPath string) (string, error) {
	if u == nil || u.client == nil {
		return "", errors.New("upload: kickoff: uploader/client is nil")
	}

	cleanPath = strings.TrimSpace(cleanPath)
	if cleanPath == "" {
		if _, has := body["data"]; !has {
			return "", errors.New("upload: kickoff: missing local file path and 'data'")
		}
	}

	var resp UploadResponse
	path := utils.ProjectPath(u.client.ProjectID, "files/upload")

	factory := uploadBodyFactory{
		ctx:      ctx,
		params:   body,
		readPath: cleanPath,
	}

	if err := u.client.DoJSONWithRetry(ctx, http.MethodPost, path, factory, &resp); err != nil {
		return "", fmt.Errorf("upload: kickoff: %w", err)
	}

	processID := strings.TrimSpace(resp.Process.ProcessID)
	if processID == "" {
		return "", ErrNoProcessID
	}
	return processID, nil
}
