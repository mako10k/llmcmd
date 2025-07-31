package parser

import (
	"fmt"
	"strings"
	"unicode"
)

// TokenType represents the type of a token
type TokenType int

const (
	// Basic tokens
	WORD TokenType = iota
	PIPE           // |
	REDIRECT_OUT   // >
	REDIRECT_APPEND // >>
	REDIRECT_IN    // <
	REDIRECT_ERR   // 2>
	REDIRECT_ALL   // &>
	AND            // &&
	OR             // ||
	SEMICOLON      // ;
	NEWLINE        // \n
	EOF
	
	// Special tokens
	QUOTED_STRING  // "string" or 'string'
	BACKGROUND     // & (not implemented for security)
)

// Token represents a single token
type Token struct {
	Type     TokenType
	Value    string
	Position int
}

// Tokenizer breaks input into tokens
type Tokenizer struct {
	input    string
	position int
	current  rune
}

// NewTokenizer creates a new tokenizer
func NewTokenizer(input string) *Tokenizer {
	t := &Tokenizer{
		input:    input,
		position: 0,
	}
	
	// Handle shebang line specially
	if strings.HasPrefix(input, "#!") {
		// Skip the entire shebang line
		for t.position < len(input) && input[t.position] != '\n' {
			t.position++
		}
		// Position at newline or EOF
	}
	
	if t.position < len(input) {
		t.current = rune(input[t.position])
	} else {
		t.current = 0
	}
	
	return t
}

// advance moves to the next character
func (t *Tokenizer) advance() {
	t.position++
	if t.position >= len(t.input) {
		t.current = 0 // EOF
	} else {
		t.current = rune(t.input[t.position])
	}
}

// peek returns the next character without advancing
func (t *Tokenizer) peek() rune {
	if t.position+1 >= len(t.input) {
		return 0
	}
	return rune(t.input[t.position+1])
}

// skipWhitespace skips spaces and tabs but not newlines
func (t *Tokenizer) skipWhitespace() {
	for t.current != 0 && (t.current == ' ' || t.current == '\t') {
		t.advance()
	}
}

// skipComment skips from # to end of line
func (t *Tokenizer) skipComment() {
	for t.current != 0 && t.current != '\n' {
		t.advance()
	}
}

// readWord reads a word token
func (t *Tokenizer) readWord() string {
	start := t.position
	for t.current != 0 && !t.isSpecialChar() && !unicode.IsSpace(t.current) {
		t.advance()
	}
	return t.input[start:t.position]
}

// readQuotedString reads a quoted string
func (t *Tokenizer) readQuotedString(quote rune) (string, error) {
	start := t.position
	t.advance() // skip opening quote
	
	var result strings.Builder
	for t.current != 0 && t.current != quote {
		if t.current == '\\' {
			t.advance()
			if t.current == 0 {
				return "", fmt.Errorf("unterminated quoted string at position %d", start)
			}
			// Handle escape sequences
			switch t.current {
			case 'n':
				result.WriteRune('\n')
			case 't':
				result.WriteRune('\t')
			case 'r':
				result.WriteRune('\r')
			case '\\':
				result.WriteRune('\\')
			case '"', '\'':
				result.WriteRune(t.current)
			default:
				result.WriteRune(t.current)
			}
		} else {
			result.WriteRune(t.current)
		}
		t.advance()
	}
	
	if t.current != quote {
		return "", fmt.Errorf("unterminated quoted string at position %d", start)
	}
	t.advance() // skip closing quote
	
	return result.String(), nil
}

// isSpecialChar checks if current character is a special shell character
func (t *Tokenizer) isSpecialChar() bool {
	switch t.current {
	case '|', '>', '<', '&', ';', '\n':
		return true
	default:
		return false
	}
}

// NextToken returns the next token
func (t *Tokenizer) NextToken() (Token, error) {
	for {
		t.skipWhitespace()
		
		if t.current == 0 {
			return Token{Type: EOF, Position: t.position}, nil
		}
		
		// Skip comments (# to end of line)
		if t.current == '#' {
			t.skipComment()
			continue
		}
		
		position := t.position
		
		switch t.current {
		case '\n':
			t.advance()
			return Token{Type: NEWLINE, Value: "\n", Position: position}, nil
			
		case ';':
			t.advance()
			return Token{Type: SEMICOLON, Value: ";", Position: position}, nil
			
		case '|':
			if t.peek() == '|' {
				t.advance()
				t.advance()
				return Token{Type: OR, Value: "||", Position: position}, nil
			}
			t.advance()
			return Token{Type: PIPE, Value: "|", Position: position}, nil
			
		case '&':
			if t.peek() == '&' {
				t.advance()
				t.advance()
				return Token{Type: AND, Value: "&&", Position: position}, nil
			}
			if t.peek() == '>' {
				t.advance()
				t.advance()
				return Token{Type: REDIRECT_ALL, Value: "&>", Position: position}, nil
			}
			// & alone is background execution (not supported for security)
			return Token{}, fmt.Errorf("background execution (&) not supported for security reasons at position %d", position)
			
		case '>':
			if t.peek() == '>' {
				t.advance()
				t.advance()
				return Token{Type: REDIRECT_APPEND, Value: ">>", Position: position}, nil
			}
			t.advance()
			return Token{Type: REDIRECT_OUT, Value: ">", Position: position}, nil
			
		case '<':
			t.advance()
			return Token{Type: REDIRECT_IN, Value: "<", Position: position}, nil
			
		case '2':
			if t.peek() == '>' {
				t.advance()
				t.advance()
				return Token{Type: REDIRECT_ERR, Value: "2>", Position: position}, nil
			}
			// Fall through to word parsing
			word := t.readWord()
			return Token{Type: WORD, Value: word, Position: position}, nil
			
		case '"', '\'':
			quote := t.current
			value, err := t.readQuotedString(quote)
			if err != nil {
				return Token{}, err
			}
			return Token{Type: QUOTED_STRING, Value: value, Position: position}, nil
			
		default:
			word := t.readWord()
			if word == "" {
				return Token{}, fmt.Errorf("unexpected character '%c' at position %d", t.current, position)
			}
			return Token{Type: WORD, Value: word, Position: position}, nil
		}
	}
}

// TokenizeAll returns all tokens from the input
func (t *Tokenizer) TokenizeAll() ([]Token, error) {
	var tokens []Token
	
	for {
		token, err := t.NextToken()
		if err != nil {
			return nil, err
		}
		
		tokens = append(tokens, token)
		
		if token.Type == EOF {
			break
		}
	}
	
	return tokens, nil
}
