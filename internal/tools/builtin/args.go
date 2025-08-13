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

// HandleHelp writes helpText to stdout when help requested and returns handled=true.
func HandleHelp(args []string, stdout io.Writer, helpText string) (handled bool, remaining []string) {
	help, a := ExtractHelp(args)
	if help {
		_, _ = stdout.Write([]byte(helpText))
		return true, a
	}
	return false, a
}
