package upload

func ExportValidateAndNormalizeStdBase64String(s string) (string, error) {
	return validateAndNormalizeStdBase64String(s)
}

func ExportNormalizeStdBase64Padding(s string, pad int) (string, error) {
	return normalizeStdBase64Padding(s, pad)
}
