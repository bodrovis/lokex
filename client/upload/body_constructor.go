package upload

import (
	"bufio"
	"context"
	"fmt"
	"io"
)

func newUploadBody(ctx context.Context, params UploadParams, cleanPath string) (io.ReadCloser, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	spec, err := parseUploadDataSpec(params)
	if err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	pr, pw := io.Pipe()
	go func() {
		var werr error
		defer func() {
			if werr != nil {
				_ = pw.CloseWithError(werr)
			} else {
				_ = pw.Close()
			}
		}()

		// Close the pipe if ctx is canceled.
		stop := context.AfterFunc(ctx, func() {
			_ = pw.CloseWithError(ctx.Err())
		})
		defer stop()

		// If already canceled, bail early (avoids noisy pipe errors).
		if err := ctx.Err(); err != nil {
			werr = err
			return
		}

		bw := bufio.NewWriterSize(pw, 256<<10)
		defer func() {
			if ferr := bw.Flush(); werr == nil && ferr != nil {
				werr = ferr
			}
		}()

		werr = writeUploadJSON(bw, params, cleanPath, spec)
	}()

	return pr, nil
}

func parseUploadDataSpec(params UploadParams) (uploadDataSpec, error) {
	var spec uploadDataSpec

	v, ok := params["data"]
	if !ok {
		spec.useFile = true
		return spec, nil
	}

	switch t := v.(type) {
	case string:
		// fail fast BEFORE we create the pipe / start goroutines / send HTTP.
		norm, err := validateAndNormalizeStdBase64String(t)
		if err != nil {
			return uploadDataSpec{}, err
		}
		spec.dataString = norm
	case []byte:
		spec.dataWasBytes = true
		spec.dataBytes = t
	default:
		return uploadDataSpec{}, fmt.Errorf("upload: 'data' must be string or []byte, got %T", v)
	}

	return spec, nil
}
