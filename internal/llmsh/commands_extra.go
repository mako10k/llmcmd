package llmsh

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

// === Additional LLM Knowledge Base Commands ===

// executeFold implements fold command (line wrapping)
func (c *Commands) executeFold(args []string, stdin io.ReadWriteCloser, stdout io.ReadWriteCloser) error {
	width := 80
	
	// Parse width argument
	for i, arg := range args {
		if arg == "-w" && i+1 < len(args) {
			if w, err := strconv.Atoi(args[i+1]); err == nil {
				width = w
			}
		}
	}
	
	// Read input
	input, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("fold: error reading input: %w", err)
	}
	
	text := string(input)
	lines := strings.Split(text, "\n")
	
	for _, line := range lines {
		for len(line) > width {
			_, err := stdout.Write([]byte(line[:width] + "\n"))
			if err != nil {
				return err
			}
			line = line[width:]
		}
		if len(line) > 0 {
			_, err := stdout.Write([]byte(line + "\n"))
			if err != nil {
				return err
			}
		}
	}
	
	return nil
}

// executeExpand implements expand command (tabs to spaces)
func (c *Commands) executeExpand(args []string, stdin io.ReadWriteCloser, stdout io.ReadWriteCloser) error {
	tabstop := 8
	
	// Parse tabstop argument
	for i, arg := range args {
		if arg == "-t" && i+1 < len(args) {
			if t, err := strconv.Atoi(args[i+1]); err == nil {
				tabstop = t
			}
		}
	}
	
	// Read input
	input, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("expand: error reading input: %w", err)
	}
	
	text := string(input)
	var result strings.Builder
	column := 0
	
	for _, r := range text {
		if r == '\t' {
			spaces := tabstop - (column % tabstop)
			for i := 0; i < spaces; i++ {
				result.WriteRune(' ')
			}
			column += spaces
		} else if r == '\n' {
			result.WriteRune(r)
			column = 0
		} else {
			result.WriteRune(r)
			column++
		}
	}
	
	_, err = stdout.Write([]byte(result.String()))
	return err
}

// executeUnexpand implements unexpand command (spaces to tabs)
func (c *Commands) executeUnexpand(args []string, stdin io.ReadWriteCloser, stdout io.ReadWriteCloser) error {
	tabstop := 8
	
	// Parse tabstop argument
	for i, arg := range args {
		if arg == "-t" && i+1 < len(args) {
			if t, err := strconv.Atoi(args[i+1]); err == nil {
				tabstop = t
			}
		}
	}
	
	// Read input
	input, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("unexpand: error reading input: %w", err)
	}
	
	text := string(input)
	lines := strings.Split(text, "\n")
	
	for i, line := range lines {
		// Simple implementation: convert leading spaces to tabs
		spaces := 0
		for _, r := range line {
			if r == ' ' {
				spaces++
			} else {
				break
			}
		}
		
		tabs := spaces / tabstop
		remainingSpaces := spaces % tabstop
		
		var result strings.Builder
		for j := 0; j < tabs; j++ {
			result.WriteRune('\t')
		}
		for j := 0; j < remainingSpaces; j++ {
			result.WriteRune(' ')
		}
		result.WriteString(line[spaces:])
		
		if i < len(lines)-1 {
			result.WriteRune('\n')
		}
		
		_, err := stdout.Write([]byte(result.String()))
		if err != nil {
			return err
		}
	}
	
	return nil
}

// === Calculation ===

// executeBc implements basic calculator
func (c *Commands) executeBc(args []string, stdin io.ReadWriteCloser, stdout, stderr io.ReadWriteCloser) error {
	// Read expression from stdin
	input, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("bc: error reading input: %w", err)
	}
	
	expression := strings.TrimSpace(string(input))
	if expression == "" {
		return nil
	}
	
	// Very basic arithmetic evaluation
	result, err := c.evaluateExpression(expression)
	if err != nil {
		return fmt.Errorf("bc: %w", err)
	}
	
	_, err = stdout.Write([]byte(fmt.Sprintf("%g\n", result)))
	return err
}

// executeDc implements basic dc (desk calculator)
func (c *Commands) executeDc(args []string, stdin io.ReadWriteCloser, stdout, stderr io.ReadWriteCloser) error {
	// Read commands from stdin
	input, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("dc: error reading input: %w", err)
	}
	
	commands := strings.TrimSpace(string(input))
	if commands == "" {
		return nil
	}
	
	// Very basic dc implementation (stack-based calculator)
	stack := []float64{}
	tokens := strings.Fields(commands)
	
	for _, token := range tokens {
		switch token {
		case "+":
			if len(stack) >= 2 {
				b := stack[len(stack)-1]
				a := stack[len(stack)-2]
				stack = stack[:len(stack)-2]
				stack = append(stack, a+b)
			}
		case "-":
			if len(stack) >= 2 {
				b := stack[len(stack)-1]
				a := stack[len(stack)-2]
				stack = stack[:len(stack)-2]
				stack = append(stack, a-b)
			}
		case "*":
			if len(stack) >= 2 {
				b := stack[len(stack)-1]
				a := stack[len(stack)-2]
				stack = stack[:len(stack)-2]
				stack = append(stack, a*b)
			}
		case "/":
			if len(stack) >= 2 {
				b := stack[len(stack)-1]
				a := stack[len(stack)-2]
				stack = stack[:len(stack)-2]
				if b != 0 {
					stack = append(stack, a/b)
				}
			}
		case "p":
			if len(stack) > 0 {
				_, err := stdout.Write([]byte(fmt.Sprintf("%g\n", stack[len(stack)-1])))
				if err != nil {
					return err
				}
			}
		default:
			if num, err := strconv.ParseFloat(token, 64); err == nil {
				stack = append(stack, num)
			}
		}
	}
	
	return nil
}

// executeExpr implements expr command
func (c *Commands) executeExpr(args []string, stdout io.ReadWriteCloser) error {
	if len(args) == 0 {
		return fmt.Errorf("expr: missing expression")
	}
	
	expression := strings.Join(args, " ")
	result, err := c.evaluateExpression(expression)
	if err != nil {
		return fmt.Errorf("expr: %w", err)
	}
	
	_, err = stdout.Write([]byte(fmt.Sprintf("%g\n", result)))
	return err
}

// === Compression ===

// executeGzip implements basic gzip compression
func (c *Commands) executeGzip(args []string, stdin io.ReadWriteCloser, stdout, stderr io.ReadWriteCloser) error {
	// For simplicity, just return an error for now
	return fmt.Errorf("gzip: compression not implemented in minimal shell")
}

// executeGunzip implements basic gzip decompression
func (c *Commands) executeGunzip(args []string, stdin io.ReadWriteCloser, stdout, stderr io.ReadWriteCloser) error {
	// For simplicity, just return an error for now
	return fmt.Errorf("gunzip: decompression not implemented in minimal shell")
}

// === Helper Functions ===

// evaluateExpression performs basic arithmetic evaluation
func (c *Commands) evaluateExpression(expr string) (float64, error) {
	expr = strings.ReplaceAll(expr, " ", "")
	
	// Very basic evaluation - only supports simple operations
	// For production use, would need a proper expression parser
	
	// Handle basic operations like "1+2", "10-5", etc.
	for _, op := range []string{"+", "-", "*", "/"} {
		if strings.Contains(expr, op) && op != "-" || (op == "-" && strings.Count(expr, "-") > 0 && !strings.HasPrefix(expr, "-")) {
			parts := strings.SplitN(expr, op, 2)
			if len(parts) == 2 {
				left, err1 := strconv.ParseFloat(parts[0], 64)
				right, err2 := strconv.ParseFloat(parts[1], 64)
				if err1 == nil && err2 == nil {
					switch op {
					case "+":
						return left + right, nil
					case "-":
						return left - right, nil
					case "*":
						return left * right, nil
					case "/":
						if right != 0 {
							return left / right, nil
						}
						return 0, fmt.Errorf("division by zero")
					}
				}
			}
		}
	}
	
	// Try to parse as a simple number
	if num, err := strconv.ParseFloat(expr, 64); err == nil {
		return num, nil
	}
	
	return 0, fmt.Errorf("invalid expression: %s", expr)
}
