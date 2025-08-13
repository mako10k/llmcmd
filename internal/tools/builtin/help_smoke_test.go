package builtin

import (
	"bytes"
	"strings"
	"testing"
)

// TestHelpSmoke validates that each builtin command returns help text with --help
// after refactor to centralized HandleHelp. Exit is skipped because it calls os.Exit.
func TestHelpSmoke(t *testing.T) {
	tests := []struct {
		name string
		fn   func([]string, *bytes.Buffer) error
	}{
		{"head", func(a []string, out *bytes.Buffer) error { return Head(a, bytes.NewBuffer(nil), out) }},
		{"tail", func(a []string, out *bytes.Buffer) error { return Tail(a, bytes.NewBuffer(nil), out) }},
		{"tee", func(a []string, out *bytes.Buffer) error { return Tee(a, bytes.NewBuffer(nil), out) }},
		{"llmcmd", func(a []string, out *bytes.Buffer) error { return Llmcmd(a, bytes.NewBuffer(nil), out) }},
		{"diff", func(a []string, out *bytes.Buffer) error { return Diff(a, bytes.NewBuffer(nil), out) }},
		{"echo", func(a []string, out *bytes.Buffer) error { return Echo(a, bytes.NewBuffer(nil), out) }},
		{"sed", func(a []string, out *bytes.Buffer) error { return Sed(a, bytes.NewBuffer(nil), out) }},
		{"wc", func(a []string, out *bytes.Buffer) error { return Wc(a, bytes.NewBuffer(nil), out) }},
		{"nl", func(a []string, out *bytes.Buffer) error { return Nl(a, bytes.NewBuffer(nil), out) }},
		{"llmsh", func(a []string, out *bytes.Buffer) error { return Llmsh(a, bytes.NewBuffer(nil), out) }},
		{"rev", func(a []string, out *bytes.Buffer) error { return Rev(a, bytes.NewBuffer(nil), out) }},
		{"cut", func(a []string, out *bytes.Buffer) error { return Cut(a, bytes.NewBuffer(nil), out) }},
		{"tr", func(a []string, out *bytes.Buffer) error { return Tr(a, bytes.NewBuffer(nil), out) }},
		{"sort", func(a []string, out *bytes.Buffer) error { return Sort(a, bytes.NewBuffer(nil), out) }},
		{"uniq", func(a []string, out *bytes.Buffer) error { return Uniq(a, bytes.NewBuffer(nil), out) }},
		{"grep", func(a []string, out *bytes.Buffer) error { return Grep(a, bytes.NewBuffer(nil), out) }},
		{"cat", func(a []string, out *bytes.Buffer) error { return Cat(a, bytes.NewBuffer(nil), out) }},
		{"patch", func(a []string, out *bytes.Buffer) error { return Patch(a, bytes.NewBuffer(nil), out) }},
	}

	for _, tc := range tests {
		// For commands that require at least one non-help arg (grep expects pattern), we skip because help flag should preempt
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			err := tc.fn([]string{"--help"}, &out)
			if err != nil {
				t.Fatalf("%s --help returned error: %v", tc.name, err)
			}
			help := out.String()
			if help == "" {
				t.Fatalf("%s --help produced empty output", tc.name)
			}
			if !strings.Contains(help, "Usage:") {
				t.Errorf("%s help missing 'Usage:' section: %q", tc.name, help)
			}
		})
	}
}
