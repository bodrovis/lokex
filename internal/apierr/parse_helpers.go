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

	switch n := v.(type) {
	case float64:
		if n != math.Trunc(n) {
			return 0, false
		}

		i, err := strconv.Atoi(strconv.FormatFloat(n, 'f', -1, 64))
		if err != nil {
			return 0, false
		}

		return i, true

	case int:
		return n, true

	case string:
		i, err := strconv.Atoi(strings.TrimSpace(n))
		if err != nil {
			return 0, false
		}

		return i, true
	}

	return 0, false
}
