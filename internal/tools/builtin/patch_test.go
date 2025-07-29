package builtin

import (
	"strings"
	"testing"
)

func TestPatch(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		input          string
		expectedOutput string
		expectedError  string
	}{
		{
			name: "basic patch",
			args: []string{},
			input: `line 1
line 2
line 3
---LLMCMD_PATCH_SEPARATOR---
@@ -2,1 +2,1 @@
-line 2
+modified line 2`,
			expectedOutput: `line 1
modified line 2
line 3`,
		},
		{
			name: "add line patch",
			args: []string{},
			input: `line 1
line 3
---LLMCMD_PATCH_SEPARATOR---
@@ -1,2 +1,3 @@
 line 1
+line 2
 line 3`,
			expectedOutput: `line 1
line 2
line 3`,
		},
		{
			name: "delete line patch",
			args: []string{},
			input: `line 1
line 2
line 3
---LLMCMD_PATCH_SEPARATOR---
@@ -1,3 +1,2 @@
 line 1
-line 2
 line 3`,
			expectedOutput: `line 1
line 3`,
		},
		{
			name: "dry-run mode success",
			args: []string{"--dry-run"},
			input: `line 1
line 2
line 3
---LLMCMD_PATCH_SEPARATOR---
@@ -2,1 +2,1 @@
-line 2
+modified line 2`,
			expectedOutput: "DRY-RUN SUCCESS: patch can be applied cleanly\n",
		},
		{
			name: "dry-run mode failure",
			args: []string{"--dry-run"},
			input: `line 1
line 2
line 3
---LLMCMD_PATCH_SEPARATOR---
@@ -2,1 +2,1 @@
-line 4
+modified line 2`,
			expectedOutput: "DRY-RUN FAILED: chunk 1 validation failed: delete mismatch at line 2: expected \"line 4\", got \"line 2\"\n",
		},
		{
			name:          "missing separator",
			args:          []string{},
			input:         "line 1\nline 2",
			expectedError: "patch: input must contain exactly one ---LLMCMD_PATCH_SEPARATOR---",
		},
		{
			name: "context mismatch",
			args: []string{},
			input: `line 1
line 2
line 3
---LLMCMD_PATCH_SEPARATOR---
@@ -2,1 +2,1 @@
-different line
+modified line 2`,
			expectedError: "patch: failed to apply patch: chunk 1 application failed: delete mismatch at line 2",
		},
		{
			name:  "help message",
			args:  []string{"--help"},
			input: "",
			expectedOutput: `patch - Apply unified diff patches to text

Usage: patch [--dry-run]

Options:
  --dry-run         Don't actually apply patch (validation only)
  --help, -h        Show this help message

Input format: original_text + ---LLMCMD_PATCH_SEPARATOR--- + patch_content
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output strings.Builder
			input := strings.NewReader(tt.input)

			err := Patch(tt.args, input, &output)

			if tt.expectedError != "" {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.expectedError)
				} else if !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("expected error containing %q, got %q", tt.expectedError, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if output.String() != tt.expectedOutput {
					t.Errorf("expected output %q, got %q", tt.expectedOutput, output.String())
				}
			}
		})
	}
}
