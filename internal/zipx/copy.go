package zipx

import (
	"fmt"
	"io"
)

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
