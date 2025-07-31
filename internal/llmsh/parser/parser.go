package parser

import (
	"fmt"
)

// Parser parses shell syntax into an AST
type Parser struct {
	tokenizer *Tokenizer
	current   Token
	position  int
}

// NewParser creates a new parser
func NewParser() *Parser {
	return &Parser{}
}

// Parse parses the input string and returns an AST
func (p *Parser) Parse(input string) (Node, error) {
	p.tokenizer = NewTokenizer(input)
	
	// Get first token
	token, err := p.tokenizer.NextToken()
	if err != nil {
		return nil, err
	}
	p.current = token
	p.position = 0
	
	// Parse the script
	return p.parseScript()
}

// advance moves to the next token
func (p *Parser) advance() error {
	token, err := p.tokenizer.NextToken()
	if err != nil {
		return err
	}
	p.current = token
	p.position++
	return nil
}

// expect checks if current token is of expected type and advances
func (p *Parser) expect(tokenType TokenType) error {
	if p.current.Type != tokenType {
		return fmt.Errorf("expected %v, got %v at position %d", tokenType, p.current.Type, p.current.Position)
	}
	return p.advance()
}

// parseScript parses the top-level script
func (p *Parser) parseScript() (Node, error) {
	var statements []Node
	
	// Skip leading newlines
	for p.current.Type == NEWLINE {
		if err := p.advance(); err != nil {
			return nil, err
		}
	}
	
	for p.current.Type != EOF {
		stmt, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		
		if stmt != nil {
			statements = append(statements, stmt)
		}
		
		// Skip statement separators
		for p.current.Type == NEWLINE || p.current.Type == SEMICOLON {
			if err := p.advance(); err != nil {
				return nil, err
			}
		}
	}
	
	if len(statements) == 0 {
		return nil, nil
	}
	
	if len(statements) == 1 {
		return statements[0], nil
	}
	
	return &ScriptNode{Statements: statements}, nil
}

// parseStatement parses a single statement (with conditionals)
func (p *Parser) parseStatement() (Node, error) {
	left, err := p.parseSequence()
	if err != nil {
		return nil, err
	}
	
	// Handle conditional operators (&& and ||)
	for p.current.Type == AND || p.current.Type == OR {
		operator := p.current.Value
		if err := p.advance(); err != nil {
			return nil, err
		}
		
		right, err := p.parseSequence()
		if err != nil {
			return nil, err
		}
		
		left = &ConditionalNode{
			Left:     left,
			Operator: operator,
			Right:    right,
		}
	}
	
	return left, nil
}

// parseSequence parses a sequence of commands separated by semicolons
func (p *Parser) parseSequence() (Node, error) {
	var commands []Node
	
	cmd, err := p.parseComplexCommand()
	if err != nil {
		return nil, err
	}
	
	if cmd != nil {
		commands = append(commands, cmd)
	}
	
	for p.current.Type == SEMICOLON {
		if err := p.advance(); err != nil {
			return nil, err
		}
		
		// Skip newlines after semicolon
		for p.current.Type == NEWLINE {
			if err := p.advance(); err != nil {
				return nil, err
			}
		}
		
		cmd, err := p.parseComplexCommand()
		if err != nil {
			return nil, err
		}
		
		if cmd != nil {
			commands = append(commands, cmd)
		}
	}
	
	if len(commands) == 0 {
		return nil, nil
	}
	
	if len(commands) == 1 {
		return commands[0], nil
	}
	
	return &SequenceNode{Commands: commands}, nil
}

// parseComplexCommand parses a pipeline with redirections
func (p *Parser) parseComplexCommand() (Node, error) {
	pipeline, err := p.parsePipeline()
	if err != nil {
		return nil, err
	}
	
	if pipeline == nil {
		return nil, nil
	}
	
	var redirections []*RedirectionNode
	
	// Parse redirections
	for p.isRedirection() {
		redir, err := p.parseRedirection()
		if err != nil {
			return nil, err
		}
		redirections = append(redirections, redir)
	}
	
	if len(redirections) == 0 {
		return pipeline, nil
	}
	
	return &ComplexCommandNode{
		Pipeline:     pipeline,
		Redirections: redirections,
	}, nil
}

// parsePipeline parses a pipeline of commands
func (p *Parser) parsePipeline() (*PipelineNode, error) {
	var commands []*CommandNode
	
	cmd, err := p.parseCommand()
	if err != nil {
		return nil, err
	}
	
	if cmd == nil {
		return nil, nil
	}
	
	commands = append(commands, cmd)
	
	for p.current.Type == PIPE {
		if err := p.advance(); err != nil {
			return nil, err
		}
		
		cmd, err := p.parseCommand()
		if err != nil {
			return nil, err
		}
		
		if cmd == nil {
			return nil, fmt.Errorf("expected command after pipe at position %d", p.current.Position)
		}
		
		commands = append(commands, cmd)
	}
	
	return &PipelineNode{Commands: commands}, nil
}

// parseCommand parses a single command with arguments
func (p *Parser) parseCommand() (*CommandNode, error) {
	if p.current.Type != WORD && p.current.Type != QUOTED_STRING {
		return nil, nil
	}
	
	name := p.current.Value
	if err := p.advance(); err != nil {
		return nil, err
	}
	
	var args []string
	
	for p.current.Type == WORD || p.current.Type == QUOTED_STRING {
		args = append(args, p.current.Value)
		if err := p.advance(); err != nil {
			return nil, err
		}
	}
	
	return &CommandNode{
		Name: name,
		Args: args,
	}, nil
}

// isRedirection checks if current token is a redirection operator
func (p *Parser) isRedirection() bool {
	switch p.current.Type {
	case REDIRECT_OUT, REDIRECT_APPEND, REDIRECT_IN, REDIRECT_ERR, REDIRECT_ALL:
		return true
	default:
		return false
	}
}

// parseRedirection parses a redirection
func (p *Parser) parseRedirection() (*RedirectionNode, error) {
	var redirType RedirectionType
	
	switch p.current.Type {
	case REDIRECT_OUT:
		redirType = RedirOut
	case REDIRECT_APPEND:
		redirType = RedirAppend
	case REDIRECT_IN:
		redirType = RedirIn
	case REDIRECT_ERR:
		redirType = RedirErr
	case REDIRECT_ALL:
		redirType = RedirAll
	default:
		return nil, fmt.Errorf("expected redirection operator at position %d", p.current.Position)
	}
	
	if err := p.advance(); err != nil {
		return nil, err
	}
	
	if p.current.Type != WORD && p.current.Type != QUOTED_STRING {
		return nil, fmt.Errorf("expected filename after redirection at position %d", p.current.Position)
	}
	
	target := p.current.Value
	if err := p.advance(); err != nil {
		return nil, err
	}
	
	return &RedirectionNode{
		Type:   redirType,
		Target: target,
	}, nil
}
