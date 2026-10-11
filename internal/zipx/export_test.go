package zipx

import (
	"archive/zip"
	"io"
	"os"
	"time"
)

type ExportZipReader interface {
	Close() error
	Files() []*zip.File
}

func ExportCopyWithLimit(
	dst io.Writer,
	src io.Reader,
	max int64,
	enabled bool,
) (int64, error) {
	return copyWithLimit(dst, src, byteLimit{
		max:     max,
		enabled: enabled,
	})
}

func ExportPrepareExtractionRoot(destDir string) (*os.Root, error) {
	return prepareExtractionRoot(destDir)
}

func ExportSetOpenZipReaderForTest(
	fn func(string) (ExportZipReader, error),
) func() {
	prev := openZipReader

	openZipReader = func(path string) (zipReader, error) {
		return fn(path)
	}

	return func() {
		openZipReader = prev
	}
}

func ExportExtractEntry(
	f *zip.File,
	root *os.Root,
	p Policy,
	totalWritten int64,
) (int64, error) {
	return extractEntry(f, root, p, entryByteLimit(p, totalWritten))
}

func ExportPrepareEntryTarget(
	f *zip.File,
	p Policy,
	totalWritten int64,
) (string, os.FileMode, bool, error) {
	return prepareEntryTarget(f, entryByteLimit(p, totalWritten))
}

func ExportExtractDirEntry(
	f *zip.File,
	root *os.Root,
	rel string,
	p Policy,
) error {
	return extractDirEntry(f, root, rel, p)
}

func ExportExtractRegularFileEntry(
	f *zip.File,
	root *os.Root,
	rel string,
	p Policy,
	totalWritten int64,
) (int64, error) {
	return extractRegularFileEntry(
		f, root, rel, p, entryByteLimit(p, totalWritten),
	)
}

func ExportFilePermOrDefault(mode os.FileMode) os.FileMode {
	return filePermOrDefault(mode)
}

func ExportCreateTempOutputFile(
	root *os.Root,
	rel string,
) (*os.File, string, error) {
	return createTempOutputFile(root, rel)
}

func ExportCloseWithPrecedence(
	current error,
	closers ...io.Closer,
) error {
	return closeWithPrecedence(current, closers...)
}

func ExportFinalizeExtractedFile(
	root *os.Root,
	tmp string,
	rel string,
	modified time.Time,
	preserveTimes bool,
) error {
	return finalizeExtractedFile(
		root,
		tmp,
		rel,
		modified,
		preserveTimes,
	)
}

func ExportNormalizeZipEntryPath(name string) (string, error) {
	return normalizeZipEntryPath(name)
}

func ExportExtractSymlinkEntry(
	f *zip.File,
	root *os.Root,
	rel string,
	p Policy,
	totalWritten int64,
) (int64, error) {
	return extractSymlinkEntry(
		f, root, rel, p, entryByteLimit(p, totalWritten),
	)
}

func ExportReadSymlinkTarget(
	f *zip.File,
	p Policy,
	totalWritten int64,
) (string, int64, error) {
	return readSymlinkTarget(
		f, entryByteLimit(p, totalWritten),
	)
}

func ExportValidateSymlinkTargetString(
	entryName string,
	linkTarget string,
) error {
	return validateSymlinkTargetString(entryName, linkTarget)
}

func ExportValidateSymlinkPlacement(
	entryName string,
	rel string,
	linkTarget string,
) error {
	return validateSymlinkPlacement(
		entryName,
		rel,
		linkTarget,
	)
}

func ExportSetSymlinkForTest(
	fn func(*os.Root, string, string) error,
) func() {
	prev := symlinkFn
	symlinkFn = fn

	return func() {
		symlinkFn = prev
	}
}

func ExportSetReadAllForTest(
	fn func(io.Reader) ([]byte, error),
) func() {
	prev := readAllFn
	readAllFn = fn

	return func() {
		readAllFn = prev
	}
}
