package utils

import (
	"fmt"
	"os"
)

// ParseFileMode converts a file mode string to os flags and permissions
// Supports the standard file modes: r, w, a, r+, w+, a+
func ParseFileMode(mode string) (int, os.FileMode, error) {
	var flag int
	var perm os.FileMode = 0644

	switch mode {
	case "r":
		flag = os.O_RDONLY
	case "w":
		flag = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	case "a":
		flag = os.O_WRONLY | os.O_CREATE | os.O_APPEND
	case "r+":
		flag = os.O_RDWR
	case "w+":
		flag = os.O_RDWR | os.O_CREATE | os.O_TRUNC
	case "a+":
		flag = os.O_RDWR | os.O_CREATE | os.O_APPEND
	default:
		return 0, 0, fmt.Errorf("invalid mode: %s (valid modes: r, w, a, r+, w+, a+)", mode)
	}

	return flag, perm, nil
}
