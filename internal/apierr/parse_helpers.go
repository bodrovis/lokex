package apierr

import (
	"math"
	"strconv"
	"strings"
)

func coalesce(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

func getString(m map[string]any, key string) (string, bool) {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s, true
		}
	}
	return "", false
}

func getStringOr(m map[string]any, key, def string) string {
	if s, ok := getString(m, key); ok {
		return s
	}
	return def
}

func getNumberAsInt(m map[string]any, key string) (int, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}

	return valueAsInt(v)
}

func valueAsInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true

	case string:
		i, err := strconv.Atoi(strings.TrimSpace(n))
		return i, err == nil

	case float64:
		return floatAsInt(n)
	}

	return 0, false
}

func floatAsInt(n float64) (int, bool) {
	if n != math.Trunc(n) {
		return 0, false
	}

	i, err := strconv.Atoi(strconv.FormatFloat(n, 'f', -1, 64))
	return i, err == nil
}
