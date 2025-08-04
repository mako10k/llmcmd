package app

import (
	"bufio"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
)

// Test data structures for FSProxy protocol testing

// FSProxyMessage represents a parsed protocol message
type FSProxyMessage struct {
	Command     string
	Path        string
	Mode        string
	IsTopLevel  string
	FileNo      string
	Size        string
	Data        []byte
	InputFiles  string
	OutputFiles string
	Prompt      string
	Preset      string
}

// MockVFS implements VirtualFileSystem interface for testing
type MockVFS struct {
	mu        sync.RWMutex         // Add mutex for thread safety
	files     map[string]*MockFile
	nextFD    int
	openFiles map[int]*MockFile
	isTopLevel bool
	failOpen  bool
	failRead  bool
	failWrite bool
}

type MockFile struct {
	mu        sync.RWMutex // Add mutex for thread safety
	content   []byte
	position  int
	mode      string
	closed    bool
	readOnly  bool
	writeOnly bool
}

func NewMockVFS(isTopLevel bool) *MockVFS {
	return &MockVFS{
		files:      make(map[string]*MockFile),
		openFiles:  make(map[int]*MockFile),
		nextFD:     1000,
		isTopLevel: isTopLevel,
	}
}

func (m *MockVFS) OpenFile(filename string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.failOpen {
		return nil, os.ErrNotExist
	}
	
	file := &MockFile{
		content:   []byte{},
		position:  0,
		mode:      "rw",
		readOnly:  flag == os.O_RDONLY,
		writeOnly: (flag & os.O_WRONLY) != 0,
	}
	
	m.files[filename] = file
	m.nextFD++
	m.openFiles[m.nextFD] = file
	
	return file, nil
}

func (m *MockVFS) CreateTemp(pattern string) (io.ReadWriteCloser, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	filename := "temp_" + pattern
	file := &MockFile{
		content:  []byte{},
		position: 0,
		mode:     "rw",
	}
	
	m.files[filename] = file
	m.nextFD++
	m.openFiles[m.nextFD] = file
	
	return file, filename, nil
}

func (m *MockVFS) RemoveFile(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	delete(m.files, name)
	return nil
}

func (m *MockVFS) ListFiles() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var files []string
	for name := range m.files {
		files = append(files, name)
	}
	return files
}

func (m *MockVFS) IsTopLevel() bool {
	return m.isTopLevel
}

func (m *MockFile) Read(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.closed {
		return 0, os.ErrClosed
	}
	if m.writeOnly {
		return 0, os.ErrPermission
	}
	
	available := len(m.content) - m.position
	if available == 0 {
		return 0, io.EOF
	}
	
	n = copy(p, m.content[m.position:])
	m.position += n
	return n, nil
}

func (m *MockFile) Write(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.closed {
		return 0, os.ErrClosed
	}
	if m.readOnly {
		return 0, os.ErrPermission
	}
	
	m.content = append(m.content, p...)
	return len(p), nil
}

func (m *MockFile) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.closed = true
	return nil
}

// Test helper functions

func createTestPipe(t *testing.T) (*os.File, *os.File) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	return r, w
}

func setupTestFSProxy(t *testing.T, isTopLevel bool) (*FSProxyManager, *os.File, *os.File) {
	mockVFS := NewMockVFS(isTopLevel)
	r, w := createTestPipe(t)
	
	proxy := NewFSProxyManager(mockVFS, r, true)
	return proxy, r, w
}

func sendCommand(t *testing.T, w *os.File, command string) {
	_, err := w.WriteString(command + "\n")
	if err != nil {
		t.Fatalf("Failed to send command: %v", err)
	}
}

func readResponse(t *testing.T, r *os.File) string {
	reader := bufio.NewReader(r)
	response, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}
	return strings.TrimSpace(response)
}

// TestFSProxyMessageParser_CriticalFactors implements 4-factor MVP testing approach
func TestFSProxyMessageParser_CriticalFactors(t *testing.T) {
	// Factor 1: Core Functionality - Message parsing for all commands
	t.Run("Factor1_CoreFunctionality", func(t *testing.T) {
		tests := []struct {
			name        string
			input       string
			expected    FSProxyMessage
			expectError bool
		}{
			{
				name:  "valid_open_command_top_level",
				input: "OPEN test.txt w true",
				expected: FSProxyMessage{
					Command:    "OPEN",
					Path:       "test.txt",
					Mode:       "w",
					IsTopLevel: "true",
				},
				expectError: false,
			},
			{
				name:  "valid_open_command_child_process",
				input: "OPEN temp.txt r false",
				expected: FSProxyMessage{
					Command:    "OPEN",
					Path:       "temp.txt",
					Mode:       "r",
					IsTopLevel: "false",
				},
				expectError: false,
			},
			{
				name:  "valid_read_command",
				input: "READ 1000 256",
				expected: FSProxyMessage{
					Command: "READ",
					FileNo:  "1000",
					Size:    "256",
				},
				expectError: false,
			},
			{
				name:  "valid_write_command",
				input: "WRITE 1001 5",
				expected: FSProxyMessage{
					Command: "WRITE",
					FileNo:  "1001",
					Size:    "5",
				},
				expectError: false,
			},
			{
				name:  "valid_close_command",
				input: "CLOSE 1002",
				expected: FSProxyMessage{
					Command: "CLOSE",
					FileNo:  "1002",
				},
				expectError: false,
			},
			{
				name:  "valid_llm_chat_command",
				input: "LLM_CHAT true 0 0 15 0",
				expected: FSProxyMessage{
					Command:    "LLM_CHAT",
					IsTopLevel: "true",
				},
				expectError: false,
			},
			{
				name:  "valid_llm_quota_command",
				input: "LLM_QUOTA",
				expected: FSProxyMessage{
					Command: "LLM_QUOTA",
				},
				expectError: false,
			},
			{
				name:  "valid_llm_config_command",
				input: "LLM_CONFIG",
				expected: FSProxyMessage{
					Command: "LLM_CONFIG",
				},
				expectError: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				parsed, err := parseFSProxyMessage(tt.input)
				
				if tt.expectError && err == nil {
					t.Errorf("Expected error for input '%s', but got none", tt.input)
				}
				
				if !tt.expectError && err != nil {
					t.Errorf("Unexpected error for input '%s': %v", tt.input, err)
				}
				
				if !tt.expectError {
					if parsed.Command != tt.expected.Command {
						t.Errorf("Command mismatch: got %s, want %s", parsed.Command, tt.expected.Command)
					}
					if parsed.Path != tt.expected.Path {
						t.Errorf("Path mismatch: got %s, want %s", parsed.Path, tt.expected.Path)
					}
					if parsed.Mode != tt.expected.Mode {
						t.Errorf("Mode mismatch: got %s, want %s", parsed.Mode, tt.expected.Mode)
					}
					if parsed.IsTopLevel != tt.expected.IsTopLevel {
						t.Errorf("IsTopLevel mismatch: got %s, want %s", parsed.IsTopLevel, tt.expected.IsTopLevel)
					}
				}
			})
		}
	})

	// Factor 2: Error Handling - Invalid input and edge cases
	t.Run("Factor2_ErrorHandling", func(t *testing.T) {
		errorTests := []struct {
			name          string
			input         string
			expectedError string
		}{
			{
				name:          "empty_request",
				input:         "",
				expectedError: "empty request",
			},
			{
				name:          "unknown_command",
				input:         "INVALID_CMD",
				expectedError: "unknown command",
			},
			{
				name:          "open_missing_parameters",
				input:         "OPEN",
				expectedError: "OPEN requires filename, mode, and is_top_level",
			},
			{
				name:          "open_invalid_mode",
				input:         "OPEN test.txt invalid true",
				expectedError: "invalid mode",
			},
			{
				name:          "open_invalid_top_level",
				input:         "OPEN test.txt w maybe",
				expectedError: "invalid is_top_level",
			},
			{
				name:          "read_missing_parameters",
				input:         "READ",
				expectedError: "READ requires fileno and size",
			},
			{
				name:          "read_invalid_fileno",
				input:         "READ abc 256",
				expectedError: "invalid fileno",
			},
			{
				name:          "read_invalid_size",
				input:         "READ 1000 abc",
				expectedError: "invalid size",
			},
			{
				name:          "write_missing_parameters",
				input:         "WRITE",
				expectedError: "WRITE requires fileno and size",
			},
			{
				name:          "close_missing_parameters",
				input:         "CLOSE",
				expectedError: "CLOSE requires fileno",
			},
			{
				name:          "llm_chat_missing_parameters",
				input:         "LLM_CHAT",
				expectedError: "LLM_CHAT requires is_top_level, input_files_count, output_files_count, prompt_length, and preset_length",
			},
		}

		for _, tt := range errorTests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := parseFSProxyMessage(tt.input)
				if err == nil {
					t.Errorf("Expected error for input '%s', but got none", tt.input)
				} else if !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.expectedError, err.Error())
				}
			})
		}
	})

	// Factor 3: Data Integrity - Parameter validation and format checking
	t.Run("Factor3_DataIntegrity", func(t *testing.T) {
		t.Run("mode_validation", func(t *testing.T) {
			validModes := []string{"r", "w", "a", "r+", "w+", "a+"}
			for _, mode := range validModes {
				input := "OPEN test.txt " + mode + " true"
				parsed, err := parseFSProxyMessage(input)
				if err != nil {
					t.Errorf("Valid mode '%s' should not produce error: %v", mode, err)
				}
				if parsed.Mode != mode {
					t.Errorf("Mode not preserved: got %s, want %s", parsed.Mode, mode)
				}
			}
		})

		t.Run("top_level_flag_validation", func(t *testing.T) {
			validFlags := []string{"true", "false"}
			for _, flag := range validFlags {
				input := "OPEN test.txt w " + flag
				parsed, err := parseFSProxyMessage(input)
				if err != nil {
					t.Errorf("Valid flag '%s' should not produce error: %v", flag, err)
				}
				if parsed.IsTopLevel != flag {
					t.Errorf("IsTopLevel not preserved: got %s, want %s", parsed.IsTopLevel, flag)
				}
			}
		})

		t.Run("numeric_parameter_validation", func(t *testing.T) {
			tests := []struct {
				input     string
				expectErr bool
			}{
				{"READ 1000 256", false},
				{"READ 0 0", false},
				{"READ 999999 1024", false},
				{"WRITE 1000 0", false},
				{"CLOSE 1000", false},
			}

			for _, tt := range tests {
				_, err := parseFSProxyMessage(tt.input)
				if tt.expectErr && err == nil {
					t.Errorf("Expected error for input '%s'", tt.input)
				}
				if !tt.expectErr && err != nil {
					t.Errorf("Unexpected error for input '%s': %v", tt.input, err)
				}
			}
		})
	})

	// Factor 4: Performance and Security - Input sanitization and resource limits
	t.Run("Factor4_PerformanceAndSecurity", func(t *testing.T) {
		t.Run("filename_security", func(t *testing.T) {
			securityTests := []struct {
				filename string
				allowedInTopLevel bool
				allowedInChild bool
			}{
				{"test.txt", true, true},
				{"/tmp/temp.txt", true, false},
				{"../test.txt", false, false},
				{"/etc/passwd", true, false},
				{"normal_file.log", true, true},
			}

			for _, tt := range securityTests {
				// Test with top-level context
				input := "OPEN " + tt.filename + " r true"
				_, err := parseFSProxyMessage(input)
				
				// Parser should accept the input (security is enforced at VFS level)
				if err != nil {
					t.Errorf("Parser should accept filename '%s' in top-level: %v", tt.filename, err)
				}

				// Test with child process context
				input = "OPEN " + tt.filename + " r false"
				_, err = parseFSProxyMessage(input)
				
				// Parser should accept the input (security is enforced at VFS level)
				if err != nil {
					t.Errorf("Parser should accept filename '%s' in child: %v", tt.filename, err)
				}
			}
		})

		t.Run("large_input_handling", func(t *testing.T) {
			// Test with large filename
			largeFilename := strings.Repeat("a", 1000)
			input := "OPEN " + largeFilename + " w true"
			
			parsed, err := parseFSProxyMessage(input)
			if err != nil {
				t.Errorf("Should handle large filename: %v", err)
			}
			if parsed.Path != largeFilename {
				t.Errorf("Large filename not preserved correctly")
			}
		})

		t.Run("malformed_input_resistance", func(t *testing.T) {
			malformedInputs := []string{
				"OPEN\t\t\ttest.txt\tw\ttrue",  // Tabs
				"OPEN  test.txt  w  true",        // Multiple spaces
				" OPEN test.txt w true ",         // Leading/trailing spaces
				"open test.txt w true",           // Lowercase command
			}

			for _, input := range malformedInputs {
				_, err := parseFSProxyMessage(input)
				// Some malformed inputs should be handled gracefully
				// Specific behavior depends on implementation requirements
				t.Logf("Malformed input '%s' result: %v", input, err)
			}
		})
	})
}

// Helper function for parsing protocol messages (will be implemented)
func parseFSProxyMessage(input string) (FSProxyMessage, error) {
	// This is a placeholder implementation
	// The actual implementation will be created based on test requirements
	
	input = strings.TrimSpace(input)
	if input == "" {
		return FSProxyMessage{}, &FSProxyError{Message: "empty request"}
	}
	
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return FSProxyMessage{}, &FSProxyError{Message: "empty request"}
	}
	
	command := parts[0]
	msg := FSProxyMessage{Command: command}
	
	switch command {
	case "OPEN":
		if len(parts) < 4 {
			return FSProxyMessage{}, &FSProxyError{Message: "OPEN requires filename, mode, and is_top_level"}
		}
		msg.Path = parts[1]
		msg.Mode = parts[2]
		msg.IsTopLevel = parts[3]
		
		// Validate mode
		validModes := map[string]bool{"r": true, "w": true, "a": true, "r+": true, "w+": true, "a+": true}
		if !validModes[msg.Mode] {
			return FSProxyMessage{}, &FSProxyError{Message: "invalid mode: " + msg.Mode}
		}
		
		// Validate is_top_level
		if msg.IsTopLevel != "true" && msg.IsTopLevel != "false" {
			return FSProxyMessage{}, &FSProxyError{Message: "invalid is_top_level: " + msg.IsTopLevel}
		}
		
	case "READ":
		if len(parts) < 3 {
			return FSProxyMessage{}, &FSProxyError{Message: "READ requires fileno and size"}
		}
		msg.FileNo = parts[1]
		msg.Size = parts[2]
		
		// Validate fileno (basic numeric check)
		if !isNumeric(msg.FileNo) {
			return FSProxyMessage{}, &FSProxyError{Message: "invalid fileno: " + msg.FileNo}
		}
		
		// Validate size
		if !isNumeric(msg.Size) {
			return FSProxyMessage{}, &FSProxyError{Message: "invalid size: " + msg.Size}
		}
		
	case "WRITE":
		if len(parts) < 3 {
			return FSProxyMessage{}, &FSProxyError{Message: "WRITE requires fileno and size"}
		}
		msg.FileNo = parts[1]
		msg.Size = parts[2]
		
		// Validate fileno
		if !isNumeric(msg.FileNo) {
			return FSProxyMessage{}, &FSProxyError{Message: "invalid fileno: " + msg.FileNo}
		}
		
		// Validate size
		if !isNumeric(msg.Size) {
			return FSProxyMessage{}, &FSProxyError{Message: "invalid size: " + msg.Size}
		}
		
	case "CLOSE":
		if len(parts) < 2 {
			return FSProxyMessage{}, &FSProxyError{Message: "CLOSE requires fileno"}
		}
		msg.FileNo = parts[1]
		
		// Validate fileno
		if !isNumeric(msg.FileNo) {
			return FSProxyMessage{}, &FSProxyError{Message: "invalid fileno: " + msg.FileNo}
		}
		
	case "LLM_CHAT":
		if len(parts) < 6 {
			return FSProxyMessage{}, &FSProxyError{Message: "LLM_CHAT requires is_top_level, input_files_count, output_files_count, prompt_length, and preset_length"}
		}
		msg.IsTopLevel = parts[1]
		
		// Validate is_top_level
		if msg.IsTopLevel != "true" && msg.IsTopLevel != "false" {
			return FSProxyMessage{}, &FSProxyError{Message: "invalid is_top_level: " + msg.IsTopLevel}
		}
		
	case "LLM_QUOTA", "LLM_CONFIG":
		// These commands require no parameters
		
	default:
		return FSProxyMessage{}, &FSProxyError{Message: "unknown command: " + command}
	}
	
	return msg, nil
}

// Helper function to check if string is numeric
func isNumeric(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}

// FSProxyError represents a protocol error
type FSProxyError struct {
	Message string
}

func (e *FSProxyError) Error() string {
	return e.Message
}
