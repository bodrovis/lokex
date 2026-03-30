package download

import "io"

func ExportWriteHTTPBodyAtomically(destPath string, src io.Reader, wantLen int64) error {
	return writeHTTPBodyAtomically(destPath, src, wantLen)
}

func ExportValidateBundleURL(raw string) (string, error) {
	return validateBundleURL(raw)
}
