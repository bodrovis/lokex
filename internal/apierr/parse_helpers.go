package apierr

import (
	"encoding/json"
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
	case json.Number:
		if i, err := strconv.Atoi(n.String()); err == nil {
			return i, true
		}
	case float64:
		return int(n), true
	case int:
		return n, true
	case string:
		if i, err := strconv.Atoi(strings.TrimSpace(n)); err == nil {
			return i, true
		}
	}
	return 0, false
}
