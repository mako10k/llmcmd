package parser

import (
	"testing"
)

func TestTokenizer(t *testing.T) {
	tests := []struct {
		input    string
		expected []TokenType
	}{
		{
			input:    "cat file.txt",
			expected: []TokenType{WORD, WORD, EOF},
		},
		{
			input:    "cat file.txt | grep pattern",
			expected: []TokenType{WORD, WORD, PIPE, WORD, WORD, EOF},
		},
		{
			input:    "echo \"hello world\" > output.txt",
			expected: []TokenType{WORD, QUOTED_STRING, REDIRECT_OUT, WORD, EOF},
		},
		{
			input:    "command1 && command2 || command3",
			expected: []TokenType{WORD, AND, WORD, OR, WORD, EOF},
		},
		{
			input:    "cat file1; cat file2",
			expected: []TokenType{WORD, WORD, SEMICOLON, WORD, WORD, EOF},
		},
	}
	
	for _, test := range tests {
		tokenizer := NewTokenizer(test.input)
		
		for i, expectedType := range test.expected {
			token, err := tokenizer.NextToken()
			if err != nil {
				t.Errorf("Error tokenizing '%s': %v", test.input, err)
				break
			}
			
			if token.Type != expectedType {
				t.Errorf("Test '%s' token %d: expected %v, got %v", test.input, i, expectedType, token.Type)
			}
		}
	}
}

func TestParser(t *testing.T) {
	tests := []struct {
		input       string
		expectError bool
	}{
		{
			input:       "cat file.txt",
			expectError: false,
		},
		{
			input:       "cat file.txt | grep pattern | sort",
			expectError: false,
		},
		{
			input:       "echo hello > output.txt",
			expectError: false,
		},
		{
			input:       "test -f file && echo exists || echo missing",
			expectError: false,
		},
		{
			input:       "command1; command2; command3",
			expectError: false,
		},
		{
			input:       "cat |",  // Invalid: pipe without right side
			expectError: true,
		},
		{
			input:       "echo >",  // Invalid: redirection without target
			expectError: true,
		},
	}
	
	parser := NewParser()
	
	for _, test := range tests {
		_, err := parser.Parse(test.input)
		
		if test.expectError && err == nil {
			t.Errorf("Expected error for input '%s', but got none", test.input)
		}
		
		if !test.expectError && err != nil {
			t.Errorf("Unexpected error for input '%s': %v", test.input, err)
		}
	}
}

func TestQuotedStrings(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`"hello world"`, "hello world"},
		{`'single quotes'`, "single quotes"},
		{`"escaped \"quote\""`, `escaped "quote"`},
		{`"newline\nhere"`, "newline\nhere"},
		{`"tab\there"`, "tab\there"},
	}
	
	for _, test := range tests {
		tokenizer := NewTokenizer(test.input)
		token, err := tokenizer.NextToken()
		
		if err != nil {
			t.Errorf("Error tokenizing '%s': %v", test.input, err)
			continue
		}
		
		if token.Type != QUOTED_STRING {
			t.Errorf("Expected QUOTED_STRING for '%s', got %v", test.input, token.Type)
			continue
		}
		
		if token.Value != test.expected {
			t.Errorf("Expected '%s' for input '%s', got '%s'", test.expected, test.input, token.Value)
		}
	}
}
