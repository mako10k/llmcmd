package builtin

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Head outputs the first n lines (default 10)
func Head(args []string, stdin io.Reader, stdout io.Writer) error {
	n := 10
	if len(args) > 0 && strings.HasPrefix(args[0], "-") {
		if val, err := strconv.Atoi(args[0][1:]); err == nil {
			n = val
		}
	}

	scanner := bufio.NewScanner(stdin)
	count := 0
	for scanner.Scan() && count < n {
		fmt.Fprintln(stdout, scanner.Text())
		count++
	}

	return scanner.Err()
}
