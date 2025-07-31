package commands

import (
	"io"
)

// Manager manages all command categories
type Manager struct {
	Basic       *BasicCommands
	Conversion  *ConversionCommands
	Calculation *CalculationCommands
	Split       *SplitCommands
	Encoding    *EncodingCommands
	External    *ExternalCommands
}

// NewManager creates a new command manager
func NewManager() *Manager {
	return &Manager{
		Basic:       NewBasicCommands(),
		Conversion:  NewConversionCommands(),
		Calculation: NewCalculationCommands(),
		Split:       NewSplitCommands(),
		Encoding:    NewEncodingCommands(),
		External:    NewExternalCommands(),
	}
}

// Execute executes a command by name
func (m *Manager) Execute(name string, args []string, stdin io.ReadWriteCloser, stdout io.ReadWriteCloser) error {
	switch name {
	// Basic commands
	case "echo":
		return m.Basic.ExecuteEcho(args, stdin, stdout)
	case "printf":
		return m.Basic.ExecutePrintf(args, stdin, stdout)
	case "true":
		return m.Basic.ExecuteTrue(args, stdin, stdout)
	case "false":
		return m.Basic.ExecuteFalse(args, stdin, stdout)
	case "yes":
		return m.Basic.ExecuteYes(args, stdin, stdout)
	case "basename":
		return m.Basic.ExecuteBasename(args, stdin, stdout)
	case "dirname":
		return m.Basic.ExecuteDirname(args, stdin, stdout)
	case "seq":
		return m.Basic.ExecuteSeq(args, stdin, stdout)

	// Conversion commands
	case "base64":
		return m.Conversion.ExecuteBase64(args, stdin, stdout)
	case "od":
		return m.Conversion.ExecuteOd(args, stdin, stdout)
	case "hexdump":
		return m.Conversion.ExecuteHexdump(args, stdin, stdout)
	case "fmt":
		return m.Conversion.ExecuteFmt(args, stdin, stdout)
	case "fold":
		return m.Conversion.ExecuteFold(args, stdin, stdout)
	case "expand":
		return m.Conversion.ExecuteExpand(args, stdin, stdout)
	case "unexpand":
		return m.Conversion.ExecuteUnexpand(args, stdin, stdout)

	// Calculation commands
	case "bc":
		return m.Calculation.ExecuteBc(args, stdin, stdout)
	case "dc":
		return m.Calculation.ExecuteDc(args, stdin, stdout)
	case "expr":
		return m.Calculation.ExecuteExpr(args, stdin, stdout)
	case "test", "[":
		return m.Calculation.ExecuteTest(args, stdin, stdout)

	// Split commands
	case "split":
		return m.Split.ExecuteSplit(args, stdin, stdout)
	case "join":
		return m.Split.ExecuteJoin(args, stdin, stdout)
	case "comm":
		return m.Split.ExecuteComm(args, stdin, stdout)
	case "csplit":
		return m.Split.ExecuteCsplit(args, stdin, stdout)

	// Encoding commands
	case "uuencode":
		return m.Encoding.ExecuteUuencode(args, stdin, stdout)
	case "uudecode":
		return m.Encoding.ExecuteUudecode(args, stdin, stdout)
	case "gzip":
		return m.Encoding.ExecuteGzip(args, stdin, stdout)
	case "gunzip":
		return m.Encoding.ExecuteGunzip(args, stdin, stdout)
	case "bzip2":
		return m.Encoding.ExecuteBzip2(args, stdin, stdout)
	case "bunzip2":
		return m.Encoding.ExecuteBunzip2(args, stdin, stdout)
	case "xz":
		return m.Encoding.ExecuteXz(args, stdin, stdout)
	case "unxz":
		return m.Encoding.ExecuteUnxz(args, stdin, stdout)

	default:
		// Command not found in internal implementations
		return nil
	}
}

// IsInternalCommand checks if a command is implemented internally
func (m *Manager) IsInternalCommand(name string) bool {
	internalCommands := map[string]bool{
		// Basic commands
		"echo": true, "printf": true, "true": true, "false": true,
		"yes": true, "basename": true, "dirname": true, "seq": true,

		// Conversion commands
		"base64": true, "od": true, "hexdump": true, "fmt": true,
		"fold": true, "expand": true, "unexpand": true,

		// Calculation commands
		"bc": true, "dc": true, "expr": true, "test": true, "[": true,

		// Split commands
		"split": true, "join": true, "comm": true, "csplit": true,

		// Encoding commands
		"uuencode": true, "uudecode": true, "gzip": true, "gunzip": true,
		"bzip2": true, "bunzip2": true, "xz": true, "unxz": true,
	}

	return internalCommands[name]
}
