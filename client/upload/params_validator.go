package upload

import (
	"errors"
	"maps"
	"strings"
)

// cloneAndValidateParams copies user params and validates the filename field.
func cloneAndValidateParams(params UploadParams) (UploadParams, string, error) {
	body := make(UploadParams, len(params))
	maps.Copy(body, params)

	raw, ok := body["filename"]
	if !ok {
		return nil, "", errors.New("upload: missing 'filename' param")
	}

	name, ok := raw.(string)
	if !ok {
		return nil, "", errors.New("upload: 'filename' must be a non-empty string")
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return nil, "", errors.New("upload: 'filename' must be a non-empty string")
	}

	body["filename"] = name
	return body, name, nil
}
