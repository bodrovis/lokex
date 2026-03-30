package upload

import (
	"fmt"
	"maps"
	"strings"
)

// cloneAndValidateParams copies user params and extracts a clean file path.
func cloneAndValidateParams(params UploadParams) (UploadParams, string, error) {
	body := make(UploadParams, len(params)+1)
	maps.Copy(body, params)

	raw, ok := body["filename"]
	if !ok {
		return nil, "", fmt.Errorf("upload: missing 'filename' param")
	}

	name, ok := raw.(string)
	if !ok {
		return nil, "", fmt.Errorf("upload: 'filename' must be a non-empty string")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, "", fmt.Errorf("upload: 'filename' must be a non-empty string")
	}

	body["filename"] = name

	return body, name, nil
}
