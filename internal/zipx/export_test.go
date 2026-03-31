package zipx

import (
	"archive/zip"
	"io"
	"io/fs"
	"os"
	"time"
)

type ExportZipReader interface {
	Close() error
	Files() []*zip.File
}

func ExportCopyCapped(dst io.Writer, src io.Reader, max int64) (int64, error) {
	return copyCapped(dst, src, max)
}

func ExportPrepareExtractionRoot(destDir string) (string, error) {
	return prepareExtractionRoot(destDir)
}

func ExportSetOpenZipReaderForTest(fn func(string) (ExportZipReader, error)) func() {
	prev := openZipReader
	openZipReader = func(path string) (zipReader, error) {
		return fn(path)
	}
	return func() { openZipReader = prev }
}

func ExportSetMkdirAllForTest(fn func(string, os.FileMode) error) func() {
	prev := mkdirAll
	mkdirAll = fn
	return func() { mkdirAll = prev }
}

func ExportSetAbsPathForTest(fn func(string) (string, error)) func() {
	prev := absPath
	absPath = fn
	return func() { absPath = prev }
}

func ExportSetEvalSymlinksForTest(fn func(string) (string, error)) func() {
	prev := evalSymlinks
	evalSymlinks = fn
	return func() { evalSymlinks = prev }
}

func ExportExtractEntry(f *zip.File, destDir, destReal string, p Policy) (int64, error) {
	return extractEntry(f, destDir, destReal, p)
}

func ExportPrepareEntryTarget(f *zip.File, destDir, destReal string, p Policy) (string, fs.FileInfo, os.FileMode, bool, error) {
	return prepareEntryTarget(f, destDir, destReal, p)
}

func ExportExtractDirEntry(f *zip.File, targetAbs string, p Policy) error {
	return extractDirEntry(f, targetAbs, p)
}

func ExportCheckParentSymlinks(destReal, targetAbs, entryName string) error {
	return checkParentSymlinks(destReal, targetAbs, entryName)
}

func ExportSetMkdirAllDirForTest(fn func(string, os.FileMode) error) func() {
	prev := mkdirAllDir
	mkdirAllDir = fn
	return func() {
		mkdirAllDir = prev
	}
}

func ExportSetPathHasSymlinkOutsideForTest(fn func(string, string) (bool, error)) func() {
	prev := pathHasSymlinkOutsideFn
	pathHasSymlinkOutsideFn = fn
	return func() {
		pathHasSymlinkOutsideFn = prev
	}
}

func ExportExtractRegularFileEntry(f *zip.File, targetAbs string, p Policy) (int64, error) {
	return extractRegularFileEntry(f, targetAbs, p)
}

func ExportFilePermOrDefault(mode os.FileMode) os.FileMode {
	return filePermOrDefault(mode)
}

func ExportCreateTempOutputFile(targetAbs string, perm os.FileMode) (*os.File, string, error) {
	return createTempOutputFile(targetAbs, perm)
}

func ExportCloseWithPrecedence(current error, closers ...io.Closer) error {
	return closeWithPrecedence(current, closers...)
}

func ExportFinalizeExtractedFile(tmp, targetAbs string, modified time.Time, preserveTimes bool) error {
	return finalizeExtractedFile(tmp, targetAbs, modified, preserveTimes)
}

func ExportSetCreateTempFileForTest(fn func(string, string) (*os.File, error)) func() {
	prev := createTempFile
	createTempFile = fn
	return func() { createTempFile = prev }
}

func ExportSetRemoveFileForTest(fn func(string) error) func() {
	prev := removeFile
	removeFile = fn
	return func() { removeFile = prev }
}

func ExportSetRenameFileForTest(fn func(string, string) error) func() {
	prev := renameFile
	renameFile = fn
	return func() { renameFile = prev }
}

func ExportSetChtimesFileForTest(fn func(string, time.Time, time.Time) error) func() {
	prev := chtimesFile
	chtimesFile = fn
	return func() { chtimesFile = prev }
}

func ExportIsPathWithinBase(baseAbs, absPath string) bool {
	return isPathWithinBase(baseAbs, absPath)
}

func ExportNormalizeZipEntryPath(name string) (string, error) {
	return normalizeZipEntryPath(name)
}

func ExportResolveTargetPath(destDir, destReal, rel, originalName string) (string, error) {
	return resolveTargetPath(destDir, destReal, rel, originalName)
}

func ExportSetAbsFilepathForTest(fn func(string) (string, error)) func() {
	prev := absFilepath
	absFilepath = fn
	return func() { absFilepath = prev }
}

func ExportSetIsPathWithinBaseForTest(fn func(string, string) bool) func() {
	prev := isPathWithinBaseFn
	isPathWithinBaseFn = fn
	return func() { isPathWithinBaseFn = prev }
}

func ExportPathHasSymlinkOutside(destRoot, file string) (bool, error) {
	return pathHasSymlinkOutside(destRoot, file)
}

func ExportSetFilepathRelForTest(fn func(string, string) (string, error)) func() {
	prev := filepathRelFn
	filepathRelFn = fn
	return func() { filepathRelFn = prev }
}

func ExportSetLstatForTest(fn func(string) (os.FileInfo, error)) func() {
	prev := lstatFn
	lstatFn = fn
	return func() { lstatFn = prev }
}

func ExportSetEvalSymlinksPathForTest(fn func(string) (string, error)) func() {
	prev := evalSymlinksPathFn
	evalSymlinksPathFn = fn
	return func() { evalSymlinksPathFn = prev }
}

func ExportExtractSymlinkEntry(f *zip.File, targetAbs, destReal string, p Policy) error {
	return extractSymlinkEntry(f, targetAbs, destReal, p)
}

func ExportReadSymlinkTarget(f *zip.File) (string, error) {
	return readSymlinkTarget(f)
}

func ExportValidateSymlinkTargetString(entryName, linkTarget string) error {
	return validateSymlinkTargetString(entryName, linkTarget)
}

func ExportValidateSymlinkPlacement(entryName, targetAbs, destReal, linkTarget string) error {
	return validateSymlinkPlacement(entryName, targetAbs, destReal, linkTarget)
}

func ExportSetRemovePathForTest(fn func(string) error) func() {
	prev := removePathFn
	removePathFn = fn
	return func() { removePathFn = prev }
}

func ExportSetSymlinkForTest(fn func(string, string) error) func() {
	prev := symlinkFn
	symlinkFn = fn
	return func() { symlinkFn = prev }
}

func ExportSetReadAllForTest(fn func(io.Reader) ([]byte, error)) func() {
	prev := readAllFn
	readAllFn = fn
	return func() { readAllFn = prev }
}
