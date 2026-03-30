package upload

import (
	"fmt"
	"strings"
)

func validateAndNormalizeStdBase64String(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("upload: 'data' cannot be empty")
	}

	// Std base64 cannot have length%4 == 1 (padded or not).
	switch len(s) % 4 {
	case 0, 2, 3:
		// ok
	default:
		return "", fmt.Errorf("upload: 'data' base64 length is invalid (len%%4==1)")
	}

	// Validate alphabet and padding placement.
	pad := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case 'A' <= c && c <= 'Z',
			'a' <= c && c <= 'z',
			'0' <= c && c <= '9',
			c == '+', c == '/':
			if pad != 0 {
				return "", fmt.Errorf("upload: invalid base64 padding position")
			}
		case c == '=':
			pad++
			if pad > 2 {
				return "", fmt.Errorf("upload: invalid base64 padding")
			}
		default:
			return "", fmt.Errorf("upload: 'data' contains non-base64 char %q", c)
		}
	}

	// If padding exists, it must occupy only the last pad chars.
	if pad > 0 {
		if len(s)%4 != 0 {
			return "", fmt.Errorf("upload: invalid base64 padding (length must be multiple of 4 when '=' present)")
		}
		for i := len(s) - pad; i < len(s); i++ {
			if s[i] != '=' {
				return "", fmt.Errorf("upload: invalid base64 padding")
			}
		}
		return s, nil
	}

	// No '=' padding provided -> normalize to StdEncoding by adding '='.
	if m := len(s) % 4; m != 0 {
		s += strings.Repeat("=", 4-m)
	}
	return s, nil
}
