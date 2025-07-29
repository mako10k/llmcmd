package cli

import (
	"os"
	"reflect"
	"testing"
)

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    *Config
		wantErr error
	}{
		{
			name: "basic prompt",
			args: []string{"-p", "test prompt"},
			want: &Config{
				Prompt:       "test prompt",
				InputFiles:   []string{"-"}, // Default to stdin when no input files specified
				Instructions: "",
			},
		},
		{
			name: "multiple input files",
			args: []string{"-i", "file1.txt", "-i", "file2.txt", "test instruction"},
			want: &Config{
				Prompt:       "",
				InputFiles:   []string{"file1.txt", "file2.txt"},
				Instructions: "test instruction",
			},
		},
		{
			name:    "help flag",
			args:    []string{"-h"},
			want:    nil,
			wantErr: ErrShowHelp,
		},
		{
			name:    "version flag",
			args:    []string{"--version"},
			want:    nil,
			wantErr: ErrShowVersion,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip tests that require actual files for validation
			if tt.name == "multiple input files" {
				// Create temporary files for testing
				for _, filename := range tt.want.InputFiles {
					f, err := os.Create(filename)
					if err != nil {
						t.Fatalf("Failed to create test file: %v", err)
					}
					f.Close()
					defer os.Remove(filename)
				}
			}

			got, err := ParseArgs(tt.args)
			if err != tt.wantErr {
				t.Errorf("ParseArgs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr != nil {
				return // Expected error, don't check result
			}
			if got.Prompt != tt.want.Prompt {
				t.Errorf("ParseArgs() Prompt = %v, want %v", got.Prompt, tt.want.Prompt)
			}
			// Handle nil vs empty slice comparison
			gotFiles := got.InputFiles
			wantFiles := tt.want.InputFiles
			if gotFiles == nil {
				gotFiles = []string{}
			}
			if wantFiles == nil {
				wantFiles = []string{}
			}
			if !reflect.DeepEqual(gotFiles, wantFiles) {
				t.Errorf("ParseArgs() InputFiles = %v, want %v", gotFiles, wantFiles)
			}
			if got.Instructions != tt.want.Instructions {
				t.Errorf("ParseArgs() Instructions = %v, want %v", got.Instructions, tt.want.Instructions)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Model != "gpt-4o-mini" {
		t.Errorf("DefaultConfig() Model = %v, want gpt-4o-mini", config.Model)
	}
	if config.MaxTokens != 4096 {
		t.Errorf("DefaultConfig() MaxTokens = %v, want 4096", config.MaxTokens)
	}
	if config.Temperature != 0.1 {
		t.Errorf("DefaultConfig() Temperature = %v, want 0.1", config.Temperature)
	}
	if config.MaxAPICalls != 50 {
		t.Errorf("DefaultConfig() MaxAPICalls = %v, want 50", config.MaxAPICalls)
	}
}
