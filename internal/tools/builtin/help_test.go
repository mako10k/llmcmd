package builtin

import (
	"bytes"
	"strings"
	"testing"
)

func TestGetHelp(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectedInText []string
		expectError    bool
	}{
		{
			name:           "basic_operations",
			args:           []string{"basic_operations"},
			expectedInText: []string{"BASIC_WORKFLOW", "FD_CONCEPTS", "LEARNING_PROGRESSION", "FIRST_STEPS"},
			expectError:    false,
		},
		{
			name:           "debugging",
			args:           []string{"debugging"},
			expectedInText: []string{"DEBUG_TECHNIQUES", "ERROR_HANDLING", "VIRTUAL_FILE_DEBUG", "COMMON_ERRORS"},
			expectError:    false,
		},
		{
			name:           "multiple_keys",
			args:           []string{"data_analysis", "text_processing"},
			expectedInText: []string{"BASIC_WORKFLOW", "PIPELINE_PATTERNS", "STRING_TRANSFORMATION"},
			expectError:    false,
		},
		{
			name:        "invalid_key",
			args:        []string{"invalid_key"},
			expectError: true,
		},
		{
			name:        "no_args",
			args:        []string{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := GetHelp(tt.args, nil, &buf)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			output := buf.String()

			// Check that expected text appears in output
			for _, expected := range tt.expectedInText {
				if !strings.Contains(output, expected) {
					t.Errorf("Expected output to contain %q, but it didn't.\nOutput: %s", expected, output)
				}
			}

			// Verify output structure
			if !strings.Contains(output, "USAGE INFORMATION FOR:") {
				t.Errorf("Expected output to start with 'USAGE INFORMATION FOR:', but it didn't")
			}
		})
	}
}
