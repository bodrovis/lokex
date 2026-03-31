// Package upload provides an uploader for Lokalise file imports.
//
// This file implements the upload side of lokex:
//   - POST /files/upload with a JSON body that includes either a filename
//     (we'll read & base64 it) or an explicit base64 "data" field.
//   - Optionally poll the returned process until it finishes, or return
//     immediately with the process id if polling is disabled.
package upload

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/bodrovis/lokex/v2/client"
)

// Uploader wraps a *Client to perform Lokalise file uploads.
// Construct with NewUploader; the embedded client must be non-nil.
type Uploader struct {
	client *client.Client
}

// UploadParams represents the JSON body for /files/upload.
// Callers typically provide:
//
//	filename (string) – required; path to a local file
//	lang_iso (string) – base language code
//
// You may also set "data" yourself (string base64 or []byte); if omitted,
// Upload will read the file and base64-encode it for you.
type UploadParams map[string]any

type uploadBodyFactory struct {
	ctx      context.Context
	params   UploadParams
	readPath string
}

type uploadDataSpec struct {
	useFile      bool
	dataWasBytes bool
	dataString   string
	dataBytes    []byte
}

func (f uploadBodyFactory) NewBody() (io.ReadCloser, error) {
	return newUploadBody(f.ctx, f.params, f.readPath)
}

var kickoffUploadStreamingFn = func(
	u *Uploader,
	ctx context.Context,
	body UploadParams,
	cleanPath string,
) (string, error) {
	return u.kickoffUploadStreaming(ctx, body, cleanPath)
}

// NewUploader creates a new Uploader bound to c.
func NewUploader(c *client.Client) *Uploader {
	if c == nil {
		panic("lokex/upload: nil client passed to NewUploader")
	}
	return &Uploader{
		client: c,
	}
}

var ErrNoProcessID = errors.New("upload: no process id returned")

// Upload uploads a file to Lokalise using /files/upload.
// Behavior:
//  1. Validates and cleans the input params and resolves the local read path
//     (unless data is provided explicitly), ensuring it points to a regular file.
//  2. If "data" is absent, reads the file and base64-encodes it (StdEncoding).
//     If "data" is present as []byte, it is base64-encoded; if string, it is
//     used as-is (assumed base64).
//  3. Sends POST with retry/backoff using the client's DoJSONWithRetry helper.
//  4. Returns the server-provided process id.
//
// If poll is true, it will call PollProcesses on that process and only return
// when the process reaches "finished" (otherwise it errors). If poll is false,
// it returns immediately after kickoff with the process id.
func (u *Uploader) Upload(ctx context.Context, params UploadParams, srcPath string, poll bool) (string, error) {
	if u == nil || u.client == nil {
		return "", errors.New("upload: uploader/client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	if err := ctx.Err(); err != nil {
		return "", err
	}

	body, filename, err := cloneAndValidateParams(params)
	if err != nil {
		return "", err
	}

	readPath := ""
	if _, hasData := body["data"]; !hasData {
		readPath = strings.TrimSpace(srcPath)
		if readPath == "" {
			readPath = filename
		}
		readPath = filepath.Clean(readPath)

		if err := ensureFileIsRegular(readPath); err != nil {
			return "", err
		}
	}

	processID, err := kickoffUploadStreamingFn(u, ctx, body, readPath)
	if err != nil {
		if errors.Is(err, ErrNoProcessID) {
			if poll {
				return "", fmt.Errorf("upload: polling requested but unavailable: %w", err)
			}
			return "", nil
		}
		return "", err
	}

	if !poll {
		return processID, nil
	}
	return u.pollUntilFinished(ctx, processID)
}
