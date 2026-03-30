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

	if err := validateStdBase64Length(s); err != nil {
		return "", err
	}

	pad, err := validateStdBase64CharsAndPaddingPlacement(s)
	if err != nil {
		return "", err
	}

	return normalizeStdBase64Padding(s, pad)
}

func validateStdBase64Length(s string) error {
	// Std base64 cannot have length%4 == 1 (padded or not).
	switch len(s) % 4 {
	case 0, 2, 3:
		return nil
	default:
		return fmt.Errorf("upload: 'data' base64 length is invalid (len%%4==1)")
	}
}

func validateStdBase64CharsAndPaddingPlacement(s string) (int, error) {
	pad := 0

	for i := 0; i < len(s); i++ {
		c := s[i]

		switch {
		case 'A' <= c && c <= 'Z',
			'a' <= c && c <= 'z',
			'0' <= c && c <= '9',
			c == '+', c == '/':
			if pad != 0 {
				return 0, fmt.Errorf("upload: invalid base64 padding position")
			}

		case c == '=':
			pad++
			if pad > 2 {
				return 0, fmt.Errorf("upload: invalid base64 padding")
			}

		default:
			return 0, fmt.Errorf("upload: 'data' contains non-base64 char %q", c)
		}
	}

	return pad, nil
}

func normalizeStdBase64Padding(s string, pad int) (string, error) {
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

	if m := len(s) % 4; m != 0 {
		s += strings.Repeat("=", 4-m)
	}

	return s, nil
}
