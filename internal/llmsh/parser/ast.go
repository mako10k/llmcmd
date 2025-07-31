package parser

import "io"

// AST node types for representing parsed shell commands

// Node is the base interface for all AST nodes
type Node interface {
	String() string
}

// CommandNode represents a single command with arguments
type CommandNode struct {
	Name string   // Command name
	Args []string // Arguments
}

func (c *CommandNode) String() string {
	if len(c.Args) == 0 {
		return c.Name
	}
	result := c.Name
	for _, arg := range c.Args {
		result += " " + arg
	}
	return result
}

// PipelineNode represents a series of commands connected by pipes
type PipelineNode struct {
	Commands []*CommandNode
}

func (p *PipelineNode) String() string {
	if len(p.Commands) == 0 {
		return ""
	}
	result := p.Commands[0].String()
	for i := 1; i < len(p.Commands); i++ {
		result += " | " + p.Commands[i].String()
	}
	return result
}

// RedirectionType represents the type of redirection
type RedirectionType int

const (
	RedirOut    RedirectionType = iota // >
	RedirAppend                        // >>
	RedirIn                            // <
	RedirErr                           // 2>
	RedirAll                           // &>
)

// RedirectionNode represents input/output redirection
type RedirectionNode struct {
	Type   RedirectionType
	Target string // File name or fd
}

func (r *RedirectionNode) String() string {
	switch r.Type {
	case RedirOut:
		return "> " + r.Target
	case RedirAppend:
		return ">> " + r.Target
	case RedirIn:
		return "< " + r.Target
	case RedirErr:
		return "2> " + r.Target
	case RedirAll:
		return "&> " + r.Target
	default:
		return "unknown redirection"
	}
}

// ComplexCommandNode represents a command with redirections
type ComplexCommandNode struct {
	Pipeline     *PipelineNode
	Redirections []*RedirectionNode
}

func (c *ComplexCommandNode) String() string {
	result := c.Pipeline.String()
	for _, redir := range c.Redirections {
		result += " " + redir.String()
	}
	return result
}

// ConditionalNode represents conditional execution (&& or ||)
type ConditionalNode struct {
	Left     Node
	Operator string // "&&" or "||"
	Right    Node
}

func (c *ConditionalNode) String() string {
	return c.Left.String() + " " + c.Operator + " " + c.Right.String()
}

// SequenceNode represents sequential execution (;)
type SequenceNode struct {
	Commands []Node
}

func (s *SequenceNode) String() string {
	if len(s.Commands) == 0 {
		return ""
	}
	result := s.Commands[0].String()
	for i := 1; i < len(s.Commands); i++ {
		result += "; " + s.Commands[i].String()
	}
	return result
}

// ScriptNode represents the top-level script (multiple statements)
type ScriptNode struct {
	Statements []Node
}

func (s *ScriptNode) String() string {
	if len(s.Statements) == 0 {
		return ""
	}
	result := s.Statements[0].String()
	for i := 1; i < len(s.Statements); i++ {
		result += "\n" + s.Statements[i].String()
	}
	return result
}

// VirtualFile represents a virtual file in the shell's VFS
type VirtualFile interface {
	io.ReadWriteCloser
	Name() string
}
