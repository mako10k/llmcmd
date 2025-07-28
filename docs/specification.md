# llmcmd Specification

## Overview

`llmcmd` is a command-line tool that allows LLMs to execute tasks using the OpenAI ChatCompletion API. The LLM can perform file operations and command execution using the provided tools (read, write, pipe, exit).

## Command Line Syntax

```bash
llmcmd <options> <instructions> [input_files...]
```

### Options

| Option | Long form | Description | Required |
|--------|-----------|-------------|----------|
| -p | --prompt | LLM instruction (alternative to instructions argument) | No |
| -i | --input | Input file path(s) - can be specified multiple times | No |
| -o | --output | Output file path | No |
| -v | --verbose | Verbose output mode | No |
| -V | --version | Show version information | No |
| -h | --help | Show usage information | No |

### Arguments

- `<instructions>`: Instructions for the LLM (required if -p option is not specified)
- `[input_files...]`: Input file paths that can be specified after instructions (equivalent to multiple -i options)

### Multiple Input Files

Multiple input files can be specified in the following ways:

1. **Using -i option multiple times**:
   ```bash
   llmcmd -i file1.txt -i file2.txt -o output.txt "Merge these files"
   ```

2. **Specifying files as arguments after instructions**:
   ```bash
   llmcmd "Merge these files" file1.txt file2.txt > output.txt
   ```

3. **Mixed usage**:
   ```bash
   llmcmd -i file1.txt "Process files" file2.txt file3.txt
   ```

When multiple input files are provided:
- Files are assigned sequential file descriptors starting from 3: file1.txt (fd=3), file2.txt (fd=4), etc.
- Standard descriptors: stdin (fd=0), stdout (fd=1), stderr (fd=2)
- Each file can be accessed independently using the read tool with the corresponding file descriptor
- The LLM system prompt will include information about all opened file descriptors

## Configuration

### Configuration File: `~/.llmcmdrc`

The configuration file should be written in the following format:

```bash
OPENAI_API_KEY=sk-...
LLMCMD_MAX_INPUT_BYTES=10485760
LLMCMD_MAX_OUTPUT_BYTES=10485760
LLMCMD_READ_BUFFER_SIZE=4096
LLMCMD_MAX_API_CALLS=50
LLMCMD_TIMEOUT=300
```

### Environment Variables

When the following environment variables are set, they take priority over the configuration file:

- `OPENAI_API_KEY`: OpenAI API key (required)
- `LLMCMD_MAX_INPUT_BYTES`: Maximum input file size (default: 10MB)
- `LLMCMD_MAX_OUTPUT_BYTES`: Maximum output file size (default: 10MB)
- `LLMCMD_READ_BUFFER_SIZE`: Default read buffer size (default: 4096 bytes)
- `LLMCMD_MAX_API_CALLS`: Maximum number of API calls per session (default: 50)
- `LLMCMD_TIMEOUT`: Maximum execution time in seconds (default: 300)

## LLM System Prompt

The LLM will be provided with a system prompt containing the following information:

```
You are an assistant that executes specified tasks. Use the following tools to complete the task.

Task: {prompt}

Open file descriptors:
- fd 0: stdin
- fd 1: stdout (or {output_filename})
- fd 2: stderr
- fd 3: {filename_3} ({file_size_3} bytes)
- fd 4: {filename_4} ({file_size_4} bytes)
...

Available tools:
1. read(fd, offset, max_size) - Read data from file descriptor
2. write(fd, data) - Write data to file descriptor
3. pipe(cmd, in_fd, out_fd) - Execute built-in command or transfer data
4. exit(code) - Exit program

For detailed specifications, refer to the tool definitions below.
```

## Tool Definitions

### read Tool

Read data from a file or input stream.

**Function name**: `read`

**Parameters**:
- `fd` (integer, optional): File descriptor (default: 0)
- `offset` (integer, optional): Read start position (default: 0)
- `max_size` (integer, optional): Maximum read size (default: 4096)

**Return value**:
```json
{
  "input": "read data",
  "next_offset": 1024,
  "eof": true,
  "size": 1024,
  "error": null
}
```

**Description**:
- When `fd` is 0, reads from stdin
- When `fd` is 1, cannot be used for reading (stdout)
- When `fd` is 2, cannot be used for reading (stderr)  
- When `fd` is 3 or higher, reads from the corresponding input file
- Invalid file descriptor returns error
- `offset` is cumulative position (continues from previous read position)
- `max_size` limits the amount of data read in one call (default: 4096 bytes)
- `size` in return value indicates actual bytes read

### write Tool

Write data to a file or output stream.

**Function name**: `write`

**Parameters**:
- `fd` (integer, optional): File descriptor (default: 1)
- `data` (string, required): Data to write

**Return value**:
```json
{
  "success": true,
  "size": 1024,
  "error": null
}
```

**Description**:
- When `fd` is 1, writes to output file (-o option) or stdout
- When `fd` is 0, 2, cannot be used for writing (stdin, stderr reserved)
- When `fd` is 3 or higher, cannot be used for writing (input files are read-only)
- Invalid file descriptor returns error
- Data is written in append mode
- `size` in return value indicates actual bytes written

### pipe Tool

Execute built-in command or create data transfer pipe.

**Function name**: `pipe`

**Parameters**:
- `cmd` (string, optional): Built-in command to execute (if not specified, creates a simple data transfer pipe like cat)
- `in_fd` (integer, optional): Input file descriptor
- `out_fd` (integer, optional): Output file descriptor

**Built-in Commands**:
- `cat` - Copy input to output (default when cmd is not specified)
- `grep <pattern>` - Filter lines matching pattern (basic regex support)
- `sed s/<pattern>/<replacement>/[g]` - Text substitution (basic sed functionality)
- `head [-n <num>]` - Output first N lines (default: 10)
- `tail [-n <num>]` - Output last N lines (default: 10)
- `sort` - Sort lines alphabetically
- `wc [-l|-w|-c]` - Count lines, words, or characters
- `tr <set1> <set2>` - Character translation

**Return value**:

When both `in_fd` and `out_fd` are specified (data transfer):
```json
{
  "success": true,
  "size": 1024,
  "error": null
}
```

When creating new pipe or executing command:
```json
{
  "success": true,
  "in_fd": 5,
  "out_fd": 6,
  "error": null
}
```

**Description**:
- When `cmd` is not specified, creates a simple data transfer from `in_fd` to `out_fd` (like cat)
- Built-in commands are implemented internally for security
- When new pipes are created, returned `in_fd` and `out_fd` can be used for subsequent operations
- When both `in_fd` and `out_fd` are specified, returns the number of bytes transferred

**Usage Examples**:
```bash
# Cat-like operation for multiple files
llmcmd "Same as cat" file1.txt file2.txt

# LLM response:
pipe(3, 1) -> {"success": true, "size": 1024}
pipe(4, 1) -> {"success": true, "size": 512}
exit(0)

# Text processing with built-in commands
llmcmd "Filter and count lines containing 'error'" access.log

# LLM response:
pipe("grep error", 3, 5) -> {"success": true, "in_fd": 5, "out_fd": 6}
pipe("wc -l", 6, 1) -> {"success": true, "size": 25}
exit(0)
```

### exit Tool

Exit the program with specified exit code.

**Function name**: `exit`

**Parameters**:
- `code` (integer, optional): Exit code (default: 0)

**Return value**: None (program exits)

## Error Handling

### General Errors

- If configuration file cannot be read, output warning and operate with environment variables only
- If OpenAI API key is not set, exit with error
- If file size limit is exceeded, exit with error
- If maximum API calls limit is reached, exit with error
- If execution timeout is reached, exit with error

### Tool Execution Errors

- Request to execute unsupported built-in commands
- Invalid command syntax or parameters
- File access permission errors
- Insufficient disk space
- Invalid file descriptor access
- Read/write operation failures

## Security

### Built-in Command Security

All text processing commands are implemented as built-in functions to avoid security risks:

- No external command execution
- No file system access beyond specified input/output files
- Built-in commands: `cat`, `grep`, `sed`, `head`, `tail`, `sort`, `wc`, `tr`
- Commands operate only on data streams, not directly on files
- No shell metacharacter interpretation
- No environment variable access from commands

### API Call Limits

- Maximum API calls per session: `LLMCMD_MAX_API_CALLS` (default: 50)
- Execution timeout: `LLMCMD_TIMEOUT` (default: 300 seconds)
- Prevents infinite loops and excessive token usage

### File Size Limits

- Input files: `LLMCMD_MAX_INPUT_BYTES` (default: 10MB)
- Output files: `LLMCMD_MAX_OUTPUT_BYTES` (default: 10MB)

### Path Restrictions

- Relative paths are processed relative to the current directory
- Absolute paths have no restrictions (depends on filesystem permissions)

## Usage Examples

### File Conversion

```bash
llmcmd -i input.txt -o output.json "Convert text file to JSON format"
```

### Multiple File Processing

```bash
llmcmd "Merge these files" file1.txt file2.txt file3.txt > merged.txt
```

### Data Processing Pipeline

```bash
llmcmd -i data.csv "Analyze CSV file and output statistics"
```

### Standard Input/Output Usage

```bash
echo "Hello World" | llmcmd "Convert input to uppercase"
```

## Implementation Specifications

### Project Structure

```
llmcmd/
├── cmd/
│   └── llmcmd/
│       └── main.go
├── internal/
│   ├── config/
│   │   └── config.go
│   ├── openai/
│   │   └── client.go
│   ├── tools/
│   │   ├── read.go
│   │   ├── write.go
│   │   ├── pipe.go
│   │   ├── exit.go
│   │   └── builtin/
│   │       ├── cat.go
│   │       ├── grep.go
│   │       ├── sed.go
│   │       ├── head.go
│   │       ├── tail.go
│   │       ├── sort.go
│   │       ├── wc.go
│   │       └── tr.go
│   └── cli/
│       └── parser.go
├── go.mod
├── go.sum
└── README.md
```

### OpenAI API Usage Specifications

- Model: `gpt-4o` or `gpt-4o-mini`
- Feature: Function Calling (Tools)
- Max tokens: 4096
- Temperature: 0.1 (consistency focused)

### Error Codes

- 0: Normal exit
- 1: General error
- 2: Configuration error
- 3: API error
- 4: File access error
- 5: Security error
- 6: Timeout error
- 7: API call limit exceeded

## Development Constraints

### Language Policy

- **Codebase and commands**: English only
- **Runtime input**: Japanese input is allowed for user instructions
- **Comments and documentation**: English preferred, Japanese acceptable for implementation notes
