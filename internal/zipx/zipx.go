// Package zipx provides safe ZIP validation and extraction with limits
// against zip-slip, oversized archives, and special files.
package zipx

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Policy defines extraction limits and behavior.
type Policy struct {
	MaxFiles      int   // maximum number of files allowed
	MaxTotalBytes int64 // maximum total uncompressed bytes
	MaxFileBytes  int64 // maximum size per file
	AllowSymlinks bool  // whether symlinks are allowed
	PreserveTimes bool  // whether to preserve file mtimes
}

// DefaultPolicy returns conservative defaults: 20k files,
// 2 GiB total, 512 MiB per file, no symlinks, no times.
func DefaultPolicy() Policy {
	return Policy{
		MaxFiles:      20000,
		MaxTotalBytes: 2 << 30,   // 2 GiB
		MaxFileBytes:  512 << 20, // 512 MiB
	}
}

// Validate checks that zipPath is a readable ZIP file.
// Returns io.ErrUnexpectedEOF if it is not.
func Validate(zipPath string) (err error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		if errors.Is(err, zip.ErrFormat) || errors.Is(err, io.ErrUnexpectedEOF) {
			return fmt.Errorf("zip validate: %w", io.ErrUnexpectedEOF)
		}
		return fmt.Errorf("zip validate open: %w", err)
	}
	defer func() {
		if cerr := zr.Close(); err == nil && cerr != nil {
			err = fmt.Errorf("zip validate close: %w", cerr)
		}
	}()

	return nil
}

// Unzip extracts srcZip into destDir according to policy p.
// It enforces limits, prevents zip-slip, and skips unsafe entries.
func Unzip(srcZip, destDir string, p Policy) error {
	r, err := zip.OpenReader(srcZip)
	if err != nil {
		return err
	}

	defer func() {
		if cerr := r.Close(); cerr != nil {
			err = errors.Join(err, fmt.Errorf("close zip: %w", cerr))
		}
	}()

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}

	destAbs, err := filepath.Abs(destDir)
	if err != nil {
		return err
	}

	if len(r.File) > p.MaxFiles {
		return fmt.Errorf("zip too many files: %d", len(r.File))
	}

	var total int64
	for _, f := range r.File {
		if f.UncompressedSize64 > uint64(p.MaxFileBytes) {
			return fmt.Errorf("zip entry too big: %s (%d bytes)", f.Name, f.UncompressedSize64)
		}
		total += int64(f.UncompressedSize64)
		if total > p.MaxTotalBytes {
			return fmt.Errorf("zip too large uncompressed: %d", total)
		}

		rel := path.Clean(f.Name)
		rel = strings.TrimPrefix(rel, "/")
		rel = strings.TrimPrefix(rel, "./")
		if rel == "." || rel == "" {
			continue
		}
		targetPath := filepath.Join(destDir, rel)

		targetAbs, err := filepath.Abs(targetPath)
		if err != nil {
			return err
		}
		if targetAbs != destAbs && !strings.HasPrefix(targetAbs, destAbs+string(filepath.Separator)) {
			return fmt.Errorf("unsafe path in zip: %q", f.Name)
		}

		info := f.FileInfo()
		if info.IsDir() {
			if err := os.MkdirAll(targetAbs, 0o755); err != nil {
				return err
			}
			continue
		}

		mode := info.Mode()
		if !p.AllowSymlinks && (mode&os.ModeSymlink != 0) {
			continue
		}
		if mode&(os.ModeDevice|os.ModeNamedPipe|os.ModeSocket) != 0 {
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetAbs), 0o755); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		perm := mode.Perm()
		if perm == 0 {
			perm = 0o644
		}

		out, err := os.OpenFile(targetAbs, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
		if err != nil {
			_ = rc.Close()
			return err
		}

		written, copyErr := copyCapped(out, rc, p.MaxFileBytes)
		defer func() { _ = rc.Close() }()
		if cerr := out.Close(); copyErr == nil && cerr != nil {
			copyErr = cerr
		}
		if copyErr != nil {
			return copyErr
		}

		_ = written

		if p.PreserveTimes {
			if !f.Modified.IsZero() {
				_ = os.Chtimes(targetAbs, f.Modified, f.Modified)
			}
		}
	}
	return nil
}

// copyCapped copies from src to dst up to max bytes,
// returning an error if max is exceeded.
func copyCapped(dst io.Writer, src io.Reader, max int64) (int64, error) {
	if max > 0 {
		lr := &io.LimitedReader{R: src, N: max + 1}
		n, err := io.Copy(dst, lr)
		if err != nil {
			return n, err
		}
		if lr.N == 0 {
			return n, fmt.Errorf("zip entry exceeds max size")
		}
		return n, nil
	}
	return io.Copy(dst, src)
}
