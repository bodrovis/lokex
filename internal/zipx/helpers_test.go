package zipx_test

import (
	"archive/zip"
	"os"
	"strings"
	"testing"
	"time"
)

func contains(s, sub string) bool { return strings.Contains(s, sub) }

type zentry struct {
	name     string
	data     []byte
	mode     os.FileMode
	modified time.Time
	isDir    bool
}

func makeZip(t *testing.T, entries []zentry) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "zipx-*.zip")
	if err != nil {
		t.Fatalf("create temp zip: %v", err)
	}
	defer func() { _ = f.Close() }()

	zw := zip.NewWriter(f)
	for _, e := range entries {
		name := e.name
		if e.isDir && name[len(name)-1] != '/' {
			name += "/"
		}
		h := &zip.FileHeader{
			Name:     name,
			Modified: e.modified,
			Method:   zip.Store, // keep simple, small fixtures
		}
		if e.isDir {
			h.SetMode(os.ModeDir | 0o755)
		} else if e.mode != 0 {
			h.SetMode(e.mode)
		} else {
			h.SetMode(0o644)
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatalf("create header: %v", err)
		}
		if !e.isDir && len(e.data) > 0 {
			if _, err := w.Write(e.data); err != nil {
				t.Fatalf("write entry: %v", err)
			}
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return f.Name()
}
