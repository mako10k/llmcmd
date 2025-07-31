package llmsh

import (
	"fmt"
	"sort"
	"strings"
)

// HelpSystem provides integrated help for all commands
type HelpSystem struct {
	commands map[string]*CommandHelp
}

// CommandHelp contains help information for a command
type CommandHelp struct {
	Name        string
	Usage       string
	Description string
	Options     []Option
	Examples    []Example
	Related     []string
}

// Option represents a command option
type Option struct {
	Flag        string
	Description string
}

// Example represents a usage example
type Example struct {
	Command     string
	Description string
}

// NewHelpSystem creates a new help system
func NewHelpSystem() *HelpSystem {
	h := &HelpSystem{
		commands: make(map[string]*CommandHelp),
	}

	h.initializeBuiltinHelp()
	h.initializeLLMHelp()
	h.initializeSpecialHelp()

	return h
}

// GetHelp returns help information for a command
func (h *HelpSystem) GetHelp(command string) (*CommandHelp, error) {
	if help, exists := h.commands[command]; exists {
		return help, nil
	}

	return nil, fmt.Errorf("no help available for command: %s", command)
}

// ListCommands returns a list of all available commands
func (h *HelpSystem) ListCommands() []string {
	var commands []string
	for name := range h.commands {
		commands = append(commands, name)
	}
	sort.Strings(commands)
	return commands
}

// FormatHelp formats help information for display
func (h *HelpSystem) FormatHelp(command string) (string, error) {
	help, err := h.GetHelp(command)
	if err != nil {
		return "", err
	}

	var result strings.Builder

	result.WriteString(fmt.Sprintf("NAME\n    %s - %s\n\n", help.Name, help.Description))
	result.WriteString(fmt.Sprintf("USAGE\n    %s\n\n", help.Usage))

	if len(help.Options) > 0 {
		result.WriteString("OPTIONS\n")
		for _, opt := range help.Options {
			result.WriteString(fmt.Sprintf("    %-12s %s\n", opt.Flag, opt.Description))
		}
		result.WriteString("\n")
	}

	if len(help.Examples) > 0 {
		result.WriteString("EXAMPLES\n")
		for _, ex := range help.Examples {
			result.WriteString(fmt.Sprintf("    %s\n", ex.Command))
			result.WriteString(fmt.Sprintf("        %s\n\n", ex.Description))
		}
	}

	if len(help.Related) > 0 {
		result.WriteString(fmt.Sprintf("SEE ALSO\n    %s\n", strings.Join(help.Related, ", ")))
	}

	return result.String(), nil
}

// FormatCommandList formats the list of all commands
func (h *HelpSystem) FormatCommandList() string {
	var result strings.Builder

	result.WriteString("LLMSH - Minimal Shell for LLM Text Processing\n\n")
	result.WriteString("AVAILABLE COMMANDS\n\n")

	categories := map[string][]string{
		"Built-in Text Processing": {},
		"Basic Utilities":          {},
		"Data Conversion":          {},
		"Calculation":              {},
		"Compression":              {},
		"Special Commands":         {},
	}

	builtins := []string{"cat", "grep", "sed", "head", "tail", "sort", "wc", "tr", "cut", "uniq", "nl", "tee", "rev", "diff", "patch"}
	utilities := []string{"echo", "printf", "true", "false", "test", "[", "yes", "basename", "dirname", "seq"}
	conversion := []string{"od", "hexdump", "base64", "uuencode", "uudecode", "fmt", "fold", "expand", "unexpand", "join", "comm", "csplit", "split"}
	calculation := []string{"bc", "dc", "expr"}
	compression := []string{"gzip", "gunzip", "bzip2", "bunzip2", "xz", "unxz"}
	special := []string{"llmcmd", "llmsh", "help", "man"}

	categories["Built-in Text Processing"] = builtins
	categories["Basic Utilities"] = utilities
	categories["Data Conversion"] = conversion
	categories["Calculation"] = calculation
	categories["Compression"] = compression
	categories["Special Commands"] = special

	for category, commands := range categories {
		result.WriteString(fmt.Sprintf("%s:\n", category))
		for i, cmd := range commands {
			if i%6 == 0 && i > 0 {
				result.WriteString("\n")
			}
			result.WriteString(fmt.Sprintf("    %-12s", cmd))
		}
		result.WriteString("\n\n")
	}

	result.WriteString("For help on a specific command, use:\n")
	result.WriteString("    help <command>\n")
	result.WriteString("    man <command>\n")
	result.WriteString("    <command> --help\n")

	return result.String()
}

// initializeBuiltinHelp sets up help for built-in commands
func (h *HelpSystem) initializeBuiltinHelp() {
	h.commands["cat"] = &CommandHelp{
		Name:        "cat",
		Usage:       "cat [file...]",
		Description: "concatenate files and print on stdout",
		Examples: []Example{
			{"cat file.txt", "Display contents of file.txt"},
			{"cat file1.txt file2.txt", "Concatenate multiple files"},
		},
		Related: []string{"head", "tail", "less"},
	}

	h.commands["grep"] = &CommandHelp{
		Name:        "grep",
		Usage:       "grep [options] pattern [file...]",
		Description: "search text using patterns",
		Options: []Option{
			{"-i", "ignore case"},
			{"-v", "invert match"},
			{"-n", "show line numbers"},
			{"-c", "count matches"},
		},
		Examples: []Example{
			{"grep \"error\" log.txt", "Find lines containing 'error'"},
			{"cat file.txt | grep -i \"warning\"", "Case-insensitive search"},
		},
		Related: []string{"sed", "awk"},
	}

	h.commands["sed"] = &CommandHelp{
		Name:        "sed",
		Usage:       "sed 's/pattern/replacement/flags' [file...]",
		Description: "stream editor for filtering and transforming text",
		Examples: []Example{
			{"sed 's/old/new/g' file.txt", "Replace all occurrences of 'old' with 'new'"},
			{"echo \"hello\" | sed 's/h/H/'", "Replace first 'h' with 'H'"},
		},
		Related: []string{"grep", "tr"},
	}

	// Add more built-in commands...
	h.addMoreBuiltinHelp()
}

// initializeLLMHelp sets up help for LLM-based commands
func (h *HelpSystem) initializeLLMHelp() {
	h.commands["echo"] = &CommandHelp{
		Name:        "echo",
		Usage:       "echo [string...]",
		Description: "display a line of text",
		Options: []Option{
			{"-n", "do not output trailing newline"},
		},
		Examples: []Example{
			{"echo \"Hello World\"", "Print Hello World"},
			{"echo -n \"No newline\"", "Print without newline"},
		},
		Related: []string{"printf"},
	}

	h.commands["test"] = &CommandHelp{
		Name:        "test",
		Usage:       "test expression",
		Description: "evaluate conditional expressions",
		Options: []Option{
			{"-z string", "true if string is empty"},
			{"-n string", "true if string is not empty"},
			{"string1 = string2", "true if strings are equal"},
			{"-f file", "true if file exists (limited support)"},
		},
		Examples: []Example{
			{"test -z \"$var\"", "Check if variable is empty"},
			{"test \"hello\" = \"hello\"", "String comparison"},
		},
		Related: []string{"["},
	}

	// Add more LLM commands...
	h.addMoreLLMHelp()
}

// initializeSpecialHelp sets up help for special commands
func (h *HelpSystem) initializeSpecialHelp() {
	h.commands["llmcmd"] = &CommandHelp{
		Name:        "llmcmd",
		Usage:       "llmcmd \"prompt\"",
		Description: "execute LLM processing with gpt-4o-mini",
		Examples: []Example{
			{"cat doc.txt | llmcmd \"summarize this\"", "Summarize document content"},
			{"echo \"data\" | llmcmd \"analyze this data\"", "Analyze input data"},
		},
		Related: []string{"llmsh"},
	}

	h.commands["llmsh"] = &CommandHelp{
		Name:        "llmsh",
		Usage:       "llmsh [script]",
		Description: "execute llmsh subshell commands",
		Examples: []Example{
			{"echo \"cat file | grep pattern\" | llmsh", "Execute shell pipeline"},
		},
		Related: []string{"llmcmd"},
	}

	h.commands["help"] = &CommandHelp{
		Name:        "help",
		Usage:       "help [command]",
		Description: "display help information",
		Examples: []Example{
			{"help", "List all available commands"},
			{"help grep", "Get help for grep command"},
		},
		Related: []string{"man"},
	}

	h.commands["man"] = &CommandHelp{
		Name:        "man",
		Usage:       "man command",
		Description: "display manual page (same as help)",
		Examples: []Example{
			{"man cat", "Show manual for cat command"},
		},
		Related: []string{"help"},
	}
}

// addMoreBuiltinHelp adds help for remaining built-in commands
func (h *HelpSystem) addMoreBuiltinHelp() {
	// Add help for head, tail, sort, wc, tr, cut, uniq, nl, tee, rev, diff, patch
	h.commands["head"] = &CommandHelp{
		Name:        "head",
		Usage:       "head [-n lines] [file...]",
		Description: "output the first part of files",
		Options: []Option{
			{"-n N", "output first N lines"},
		},
		Examples: []Example{
			{"head -10 file.txt", "Show first 10 lines"},
		},
		Related: []string{"tail", "cat"},
	}

	h.commands["tail"] = &CommandHelp{
		Name:        "tail",
		Usage:       "tail [-n lines] [file...]",
		Description: "output the last part of files",
		Options: []Option{
			{"-n N", "output last N lines"},
		},
		Examples: []Example{
			{"tail -20 log.txt", "Show last 20 lines"},
		},
		Related: []string{"head", "cat"},
	}

	// Add more as needed...
}

// addMoreLLMHelp adds help for remaining LLM commands
func (h *HelpSystem) addMoreLLMHelp() {
	h.commands["printf"] = &CommandHelp{
		Name:        "printf",
		Usage:       "printf format [arguments...]",
		Description: "format and print data",
		Examples: []Example{
			{"printf \"Hello %s\\n\" \"World\"", "Formatted output"},
		},
		Related: []string{"echo"},
	}

	h.commands["base64"] = &CommandHelp{
		Name:        "base64",
		Usage:       "base64 [-d] [file]",
		Description: "base64 encode/decode data",
		Options: []Option{
			{"-d", "decode data"},
		},
		Examples: []Example{
			{"echo \"hello\" | base64", "Encode text"},
			{"echo \"aGVsbG8K\" | base64 -d", "Decode text"},
		},
		Related: []string{"od", "hexdump"},
	}

	// Add more as needed...
}
