package apierr

import "encoding/json"

func ExportCoalesce(ss ...string) string {
	return coalesce(ss...)
}

func ExportGetString(m map[string]any, key string) (string, bool) {
	return getString(m, key)
}

func ExportGetStringOr(m map[string]any, key, def string) string {
	return getStringOr(m, key, def)
}

func ExportGetNumberAsInt(m map[string]any, key string) (int, bool) {
	return getNumberAsInt(m, key)
}

func ExportJSONNumber(s string) json.Number {
	return json.Number(s)
}
