package zipx

import (
	"fmt"
	"io"
)

type byteLimit struct {
	max     int64
	enabled bool
}

func entryByteLimit(p Policy, totalWritten int64) byteLimit {
	var limit byteLimit

	if p.MaxFileBytes > 0 {
		limit = byteLimit{
			max:     p.MaxFileBytes,
			enabled: true,
		}
	}

	if p.MaxTotalBytes > 0 {
		remaining := p.MaxTotalBytes - totalWritten

		if !limit.enabled || remaining < limit.max {
			limit = byteLimit{
				max:     remaining,
				enabled: true,
			}
		}
	}

	return limit
}

func copyWithLimit(
	dst io.Writer,
	src io.Reader,
	limit byteLimit,
) (int64, error) {
	if !limit.enabled {
		return io.Copy(dst, src)
	}

	n, err := io.Copy(dst, io.LimitReader(src, limit.max))
	if err != nil {
		return n, err
	}

	if n < limit.max {
		return n, nil
	}

	var extra [1]byte
	_, err = io.ReadFull(src, extra[:])

	if err == io.EOF {
		return n, nil
	}
	if err != nil {
		return n, err
	}

	return n, fmt.Errorf("zip entry exceeds max size")
}
