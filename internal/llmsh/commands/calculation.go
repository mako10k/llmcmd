package commands

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

// CalculationCommands contains mathematical calculation commands
type CalculationCommands struct{}

// NewCalculationCommands creates a new CalculationCommands instance
func NewCalculationCommands() *CalculationCommands {
	return &CalculationCommands{}
}

// ExecuteBc implements bc command (calculator)
func (c *CalculationCommands) ExecuteBc(args []string, stdin io.ReadWriteCloser, stdout io.ReadWriteCloser) error {
	input, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("bc: error reading input: %w", err)
	}

	expression := strings.TrimSpace(string(input))
	if expression == "" {
		return nil
	}

	// Simple expression evaluator for basic arithmetic
	result, err := c.evaluateExpression(expression)
	if err != nil {
		return fmt.Errorf("bc: %w", err)
	}

	_, err = stdout.Write([]byte(fmt.Sprintf("%.10g\n", result)))
	return err
}

// ExecuteDc implements dc command (RPN calculator)
func (c *CalculationCommands) ExecuteDc(args []string, stdin io.ReadWriteCloser, stdout io.ReadWriteCloser) error {
	input, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("dc: error reading input: %w", err)
	}

	expression := strings.TrimSpace(string(input))
	if expression == "" {
		return nil
	}

	// Simple RPN calculator
	result, err := c.evaluateRPN(expression)
	if err != nil {
		return fmt.Errorf("dc: %w", err)
	}

	_, err = stdout.Write([]byte(fmt.Sprintf("%.10g\n", result)))
	return err
}

// ExecuteExpr implements expr command (expression evaluation)
func (c *CalculationCommands) ExecuteExpr(args []string, stdin io.ReadWriteCloser, stdout io.ReadWriteCloser) error {
	if len(args) == 0 {
		return fmt.Errorf("expr: missing operand")
	}

	expression := strings.Join(args, " ")
	result, err := c.evaluateExpression(expression)
	if err != nil {
		return fmt.Errorf("expr: %w", err)
	}

	// expr outputs integers when possible
	if result == math.Trunc(result) {
		_, err = stdout.Write([]byte(fmt.Sprintf("%.0f\n", result)))
	} else {
		_, err = stdout.Write([]byte(fmt.Sprintf("%.10g\n", result)))
	}
	return err
}

// evaluateExpression evaluates a simple arithmetic expression
func (c *CalculationCommands) evaluateExpression(expr string) (float64, error) {
	// Remove spaces
	expr = strings.ReplaceAll(expr, " ", "")

	// Handle simple arithmetic operations
	// This is a simplified implementation for basic operations

	// Look for addition/subtraction (lowest precedence)
	for i := len(expr) - 1; i >= 0; i-- {
		if expr[i] == '+' && i > 0 {
			left, err := c.evaluateExpression(expr[:i])
			if err != nil {
				return 0, err
			}
			right, err := c.evaluateExpression(expr[i+1:])
			if err != nil {
				return 0, err
			}
			return left + right, nil
		}
		if expr[i] == '-' && i > 0 && expr[i-1] != '+' && expr[i-1] != '-' && expr[i-1] != '*' && expr[i-1] != '/' {
			left, err := c.evaluateExpression(expr[:i])
			if err != nil {
				return 0, err
			}
			right, err := c.evaluateExpression(expr[i+1:])
			if err != nil {
				return 0, err
			}
			return left - right, nil
		}
	}

	// Look for multiplication/division
	for i := len(expr) - 1; i >= 0; i-- {
		if expr[i] == '*' {
			left, err := c.evaluateExpression(expr[:i])
			if err != nil {
				return 0, err
			}
			right, err := c.evaluateExpression(expr[i+1:])
			if err != nil {
				return 0, err
			}
			return left * right, nil
		}
		if expr[i] == '/' {
			left, err := c.evaluateExpression(expr[:i])
			if err != nil {
				return 0, err
			}
			right, err := c.evaluateExpression(expr[i+1:])
			if err != nil {
				return 0, err
			}
			if right == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			return left / right, nil
		}
	}

	// Handle parentheses
	if strings.HasPrefix(expr, "(") && strings.HasSuffix(expr, ")") {
		return c.evaluateExpression(expr[1 : len(expr)-1])
	}

	// Handle negative numbers
	if strings.HasPrefix(expr, "-") {
		val, err := c.evaluateExpression(expr[1:])
		if err != nil {
			return 0, err
		}
		return -val, nil
	}

	// Parse as number
	return strconv.ParseFloat(expr, 64)
}

// evaluateRPN evaluates a Reverse Polish Notation expression
func (c *CalculationCommands) evaluateRPN(expr string) (float64, error) {
	tokens := strings.Fields(expr)
	stack := make([]float64, 0)

	for _, token := range tokens {
		switch token {
		case "+":
			if len(stack) < 2 {
				return 0, fmt.Errorf("insufficient operands for +")
			}
			b := stack[len(stack)-1]
			a := stack[len(stack)-2]
			stack = stack[:len(stack)-2]
			stack = append(stack, a+b)
		case "-":
			if len(stack) < 2 {
				return 0, fmt.Errorf("insufficient operands for -")
			}
			b := stack[len(stack)-1]
			a := stack[len(stack)-2]
			stack = stack[:len(stack)-2]
			stack = append(stack, a-b)
		case "*":
			if len(stack) < 2 {
				return 0, fmt.Errorf("insufficient operands for *")
			}
			b := stack[len(stack)-1]
			a := stack[len(stack)-2]
			stack = stack[:len(stack)-2]
			stack = append(stack, a*b)
		case "/":
			if len(stack) < 2 {
				return 0, fmt.Errorf("insufficient operands for /")
			}
			b := stack[len(stack)-1]
			a := stack[len(stack)-2]
			stack = stack[:len(stack)-2]
			if b == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			stack = append(stack, a/b)
		case "p":
			// Print top of stack
			if len(stack) > 0 {
				return stack[len(stack)-1], nil
			}
			return 0, fmt.Errorf("stack empty")
		default:
			// Parse as number
			num, err := strconv.ParseFloat(token, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid token: %s", token)
			}
			stack = append(stack, num)
		}
	}

	if len(stack) != 1 {
		return 0, fmt.Errorf("invalid expression")
	}

	return stack[0], nil
}

// ExecuteTest implements test command (conditional testing)
func (c *CalculationCommands) ExecuteTest(args []string, stdin io.ReadWriteCloser, stdout io.ReadWriteCloser) error {
	if len(args) == 0 {
		return fmt.Errorf("test: missing arguments")
	}

	// Handle [ command (which is same as test but needs closing ])
	if args[0] == "[" {
		if len(args) < 2 || args[len(args)-1] != "]" {
			return fmt.Errorf("test: missing closing ]")
		}
		args = args[1 : len(args)-1]
	}

	result, err := c.evaluateTestExpression(args)
	if err != nil {
		return err
	}

	if !result {
		return fmt.Errorf("test failed")
	}

	return nil
}

// evaluateTestExpression evaluates test expressions
func (c *CalculationCommands) evaluateTestExpression(args []string) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}

	if len(args) == 1 {
		// Single argument: true if non-empty string
		return args[0] != "", nil
	}

	if len(args) == 2 {
		// Unary operations
		switch args[0] {
		case "-z":
			return args[1] == "", nil
		case "-n":
			return args[1] != "", nil
		default:
			return false, fmt.Errorf("unknown unary operator: %s", args[0])
		}
	}

	if len(args) == 3 {
		// Binary operations
		left := args[0]
		op := args[1]
		right := args[2]

		switch op {
		case "=", "==":
			return left == right, nil
		case "!=":
			return left != right, nil
		case "-eq":
			l, err1 := strconv.Atoi(left)
			r, err2 := strconv.Atoi(right)
			if err1 != nil || err2 != nil {
				return false, fmt.Errorf("invalid number for -eq")
			}
			return l == r, nil
		case "-ne":
			l, err1 := strconv.Atoi(left)
			r, err2 := strconv.Atoi(right)
			if err1 != nil || err2 != nil {
				return false, fmt.Errorf("invalid number for -ne")
			}
			return l != r, nil
		case "-lt":
			l, err1 := strconv.Atoi(left)
			r, err2 := strconv.Atoi(right)
			if err1 != nil || err2 != nil {
				return false, fmt.Errorf("invalid number for -lt")
			}
			return l < r, nil
		case "-le":
			l, err1 := strconv.Atoi(left)
			r, err2 := strconv.Atoi(right)
			if err1 != nil || err2 != nil {
				return false, fmt.Errorf("invalid number for -le")
			}
			return l <= r, nil
		case "-gt":
			l, err1 := strconv.Atoi(left)
			r, err2 := strconv.Atoi(right)
			if err1 != nil || err2 != nil {
				return false, fmt.Errorf("invalid number for -gt")
			}
			return l > r, nil
		case "-ge":
			l, err1 := strconv.Atoi(left)
			r, err2 := strconv.Atoi(right)
			if err1 != nil || err2 != nil {
				return false, fmt.Errorf("invalid number for -ge")
			}
			return l >= r, nil
		default:
			return false, fmt.Errorf("unknown binary operator: %s", op)
		}
	}

	return false, fmt.Errorf("too many arguments for test")
}
