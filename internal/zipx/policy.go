package zipx

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
