package builtin

import "io"

// ExtractHelp determines if help flags are present. It keeps args unchanged for backward compatibility.
func ExtractHelp(args []string) (bool, []string) {
	for _, a := range args {
		if a == "--help" || a == "-h" {
			return true, args
		}
	}
	return false, args
}

// HandleHelp writes helpText to stdout when help requested.
// Returns handled=true if help was printed. Any write error is returned.
func HandleHelp(args []string, stdout io.Writer, helpText string) (handled bool, remaining []string, err error) {
	help, a := ExtractHelp(args)
	if help {
		if _, werr := io.WriteString(stdout, helpText); werr != nil {
			return true, a, werr
		}
		return true, a, nil
	}
	return false, a, nil
}
