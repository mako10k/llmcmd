package builtin

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// UsageData contains all subsection content and key mappings
type UsageData struct {
	Subsections map[string]string
	KeyMappings map[string][]string
}

// NewUsageData creates and initializes usage data
func NewUsageData() *UsageData {
	data := &UsageData{
		Subsections: make(map[string]string),
		KeyMappings: make(map[string][]string),
	}

	// Initialize subsection content
	data.initSubsections()

	// Initialize key mappings
	data.initKeyMappings()

	return data
}

// initSubsections initializes all subsection content
func (u *UsageData) initSubsections() {
	u.Subsections["basic_workflow"] = `MINIMAL PATTERN:
1. read(fd) - Read data
2. [process] - Process data
3. write(1, result) - Output result
4. exit(0) - Normal termination

STANDARD WORKFLOW:
1. read(fd) - Get input data
2. spawn("command") - Execute processing
3. read(out_fd) - Get processing result
4. write(1, processed_data) - Final output
5. exit(0) - Complete`

	u.Subsections["fd_concepts"] = `FD MAPPING:
- fd=0: stdin (standard input)
- fd=1: stdout (result output)
- fd=2: stderr (error/debug output)
- fd=3+: input files ($1, $2, $3...)

USAGE PATTERNS:
- read(0): Read from standard input
- read(3): Read from first input file
- write(1, data): Output result to stdout
- write(2, "debug info"): Debug info to stderr`

	u.Subsections["tool_reference"] = `read(fd, [lines], [count]) - Read data
  fd: File descriptor (0, 3, 4, 5...)
  lines: Line limit (optional)
  count: Character limit (optional)

write(fd, data, [newline], [eof]) - Write data
  fd: Output destination (1=stdout, 2=stderr, command_input)
  data: Output data
  newline: Add newline (true/false)
  eof: End-of-input signal (important for command execution)

open(path, [mode]) - Open virtual file
  path: File path
  mode: "r", "w", "a", "r+", "w+", "a+"
  return: New file descriptor

spawn(script, [in_fd], [out_fd]) - Execute shell script
  script: Shell script to execute
  in_fd: Input fd (optional)
  out_fd: Output fd (optional)
  return: {in_fd, out_fd} or {out_fd}

close(fd) - Close file descriptor
exit(code) - Terminate program (0=success, 1=error)`

	u.Subsections["spawn_commands"] = `TEXT PROCESSING:
- cat: Display/concatenate data
- grep: Pattern search/filter
- sed: String replacement/transformation
- head/tail: Line limit/range extraction
- sort: Sort
- uniq: Remove duplicates
- wc: Count (lines/words/characters)
- tr: Character transformation
- cut: Field extraction

PIPELINE EXAMPLES:
- spawn("grep ERROR | head -10"): Top 10 error lines
- spawn("sort | uniq -c"): Sort then count duplicates
- spawn("cut -d',' -f1,3 | sort"): Extract CSV columns 1,3 and sort
- spawn("tr '[:upper:]' '[:lower:]'"): Convert uppercase to lowercase`

	u.Subsections["pipeline_patterns"] = `FILTERING PIPELINE:
spawn("grep pattern | grep -v exclude | sort")
→ Pattern match → Exclude → Sort

ANALYSIS PIPELINE:
spawn("cut -d' ' -f1 | sort | uniq -c | sort -nr")
→ Field extract → Sort → Count duplicates → Sort by frequency

TRANSFORMATION PIPELINE:
spawn("sed 's/old/new/g' | tr '[:upper:]' '[:lower:]'")
→ String replace → Case conversion

AGGREGATION PIPELINE:
spawn("grep ERROR | wc -l")
→ Extract error lines → Count lines`

	u.Subsections["virtual_file_usage"] = `CREATE AND WRITE:
1. open("temp.txt", "w") → fd
2. write(fd, data)
3. close(fd)

READ ONCE (PIPE BEHAVIOR):
1. open("temp.txt", "r") → fd
2. read(fd) → data (file gets consumed)
3. File cannot be read again

MULTI-STAGE PROCESSING:
1. open("stage1", "w"), write(fd, raw_data)
2. spawn("process_command", stage1_fd) → stage2_fd
3. read(stage2_fd) → processed_data`

	u.Subsections["multi_file_handling"] = `SEQUENTIAL PROCESSING:
for i in range(3, 3+file_count):
  content = read(i)
  result = process(content)
  store_result(result)

PARALLEL COMPARISON:
file1 = read(3)
file2 = read(4)
spawn("diff file1_temp file2_temp")

MERGE PROCESSING:
all_data = ""
for fd in [3, 4, 5]:
  all_data += read(fd)
process_combined(all_data)`

	u.Subsections["error_handling"] = `GRACEFUL CONTINUATION:
try:
  result = spawn("command")
  if result == "":
    write(2, "Warning: no output from command\n")
    continue
except:
  write(2, "Error in processing, skipping\n")
  continue

ERROR COLLECTION:
open("errors.log", "w") → error_fd
for each_operation:
  try: process()
  except: write(error_fd, error_message)

VALIDATION:
if read(fd) == "":
  write(2, "No input data available\n")
  exit(1)`

	u.Subsections["pattern_matching"] = `BASIC PATTERNS:
spawn("grep 'pattern'") - Simple string
spawn("grep -E 'pattern1|pattern2'") - OR search
spawn("grep -v 'exclude'") - Exclude search

CONTEXT SEARCH:
spawn("grep -B3 -A3 'pattern'") - 3 lines before/after
spawn("grep -n 'pattern'") - With line numbers
spawn("grep -c 'pattern'") - Count only

FIELD-BASED MATCHING:
spawn("cut -d',' -f2 | grep 'pattern'") - Specific field search
spawn("grep '^ERROR' | cut -d' ' -f2-") - Line start pattern + field extract`

	u.Subsections["string_transformation"] = `SUBSTITUTION:
spawn("sed 's/old/new/g'") - Global replace
spawn("sed 's/^/prefix_/'") - Add to line start
spawn("sed 's/$/suffix/'") - Add to line end

CHARACTER CONVERSION:
spawn("tr '[:upper:]' '[:lower:]'") - Uppercase to lowercase
spawn("tr -d '\r'") - Delete characters
spawn("tr ' ' '_'") - Character replacement

ADVANCED TRANSFORMATION:
spawn("sed -n '/start/,/end/p'") - Range extraction
spawn("sed '/pattern/d'") - Delete pattern lines`

	u.Subsections["comparison_diff"] = `FILE COMPARISON:
1. open("file1_temp", "w"), write(fd, file1_data)
2. open("file2_temp", "w"), write(fd, file2_data)
3. spawn("diff file1_temp file2_temp")

LINE-BY-LINE DIFF:
spawn("diff -u file1 file2") - Unified diff format
spawn("diff --side-by-side file1 file2") - Side-by-side display

CONTENT ANALYSIS:
spawn("comm -12 file1 file2") - Common lines
spawn("comm -23 file1 file2") - Lines only in file1`

	u.Subsections["common_errors"] = `EMPTY OUTPUT:
Problem: spawn() returns empty result
Cause: Command syntax error, no data
Solution: Test with spawn("echo test") to verify spawn works

FILE CONSUMED:
Problem: read() returns EOF, "file already consumed"
Cause: PIPE behavior - file already read
Solution: Recreate virtual file

SYNTAX ERROR:
Problem: spawn() gives "command not found"
Cause: Using unavailable command
Solution: Use only built-in commands`

	u.Subsections["debug_techniques"] = `OUTPUT VERIFICATION:
1. write(2, "Debug: starting process\n")
2. result = spawn("command")
3. write(2, "Result length: " + len(result) + "\n")

STEP-BY-STEP DEBUGGING:
1. Test basic: spawn("echo test")
2. Test input: read(fd, 1) - Read one line to verify
3. Test simple command: spawn("wc -l")
4. Gradually increase complexity

VIRTUAL FILE DEBUG:
1. Create file: open("debug", "w"), write(fd, test_data)
2. Verify: open("debug", "r"), content = read(fd)
3. Check if content matches expected`

	u.Subsections["statistical_commands"] = `COUNTING:
spawn("wc -l") - Line count
spawn("wc -w") - Word count
spawn("wc -c") - Character count

FREQUENCY ANALYSIS:
spawn("sort | uniq -c") - Count occurrences
spawn("sort | uniq -c | sort -nr") - Sort by frequency
spawn("cut -d' ' -f1 | sort | uniq -c") - Field frequency

SAMPLING:
spawn("head -100") - First 100 lines
spawn("tail -100") - Last 100 lines`

	u.Subsections["filtering"] = `BASIC FILTERING:
spawn("grep pattern") - Include matching lines
spawn("grep -v pattern") - Exclude matching lines
spawn("grep -i pattern") - Case-insensitive match

RANGE FILTERING:
spawn("sed -n '10,20p'") - Lines 10-20
spawn("head -n 50 | tail -n 10") - Lines 41-50

CONDITIONAL FILTERING:
spawn("grep '^[0-9]'") - Lines starting with numbers
spawn("grep '[A-Z]'") - Lines containing uppercase
spawn("grep -E '^(ERROR|WARN)'") - Multiple patterns`

	u.Subsections["extraction_patterns"] = `FIELD EXTRACTION:
spawn("cut -d',' -f1,3") - CSV columns 1 and 3
spawn("cut -d' ' -f2-") - All fields from 2nd onward
spawn("cut -c10-20") - Characters 10-20

PATTERN EXTRACTION:
spawn("grep -o '[0-9]+\\.[0-9]+\\.[0-9]+\\.[0-9]+'") - IP addresses
spawn("grep -o '[a-zA-Z0-9._-]+@[a-zA-Z0-9._-]+'") - Email addresses

STRUCTURED EXTRACTION:
spawn("sed -n '/START/,/END/p'") - Extract between markers
spawn("grep -A5 -B5 'pattern'") - Context extraction`

	u.Subsections["complex_search"] = `MULTI-PATTERN SEARCH:
spawn("grep -E 'pattern1|pattern2|pattern3'") - Multiple patterns
spawn("grep pattern1 | grep pattern2") - AND search

ADVANCED MATCHING:
spawn("grep -o '\\w\\+' | sort | uniq") - Extract words
spawn("grep -v '^#' | grep -v '^$'") - Skip comments and empty lines`

	u.Subsections["structured_data"] = `CSV PROCESSING:
spawn("cut -d',' -f1") - First column
spawn("tail -n +2") - Skip header
spawn("sed 's/,/\\t/g'") - CSV to TSV

JSON-LIKE PROCESSING:
spawn("grep -o '\"[^\"]*\":[^,}]*'") - Extract key-value pairs
spawn("tr -d ' \\t' | grep -o '{[^}]*}'") - Extract objects

XML-LIKE PROCESSING:
spawn("grep -o '<tag>[^<]*</tag>'") - Extract tag content`

	u.Subsections["encoding_conversion"] = `CHARACTER ENCODING:
spawn("tr -d '\\r'") - Remove carriage returns

CASE CONVERSION:
spawn("tr '[:upper:]' '[:lower:]'") - Lowercase
spawn("tr '[:lower:]' '[:upper:]'") - Uppercase

FORMAT NORMALIZATION:
spawn("sed 's/[[:space:]]\\+/ /g'") - Normalize whitespace
spawn("sed 's/^[[:space:]]*//'") - Remove leading whitespace
spawn("sed 's/[[:space:]]*$//'") - Remove trailing whitespace`

	u.Subsections["time_series"] = `DATE FILTERING:
spawn("grep '2024-01'") - Filter by year-month
spawn("grep '^[0-9]\\{4\\}-[0-9]\\{2\\}-[0-9]\\{2\\}'") - Date pattern

TIME EXTRACTION:
spawn("cut -d' ' -f1") - Extract date field
spawn("cut -d'T' -f1") - Extract date from ISO format

AGGREGATION BY TIME:
spawn("cut -d' ' -f1 | sort | uniq -c") - Count by date
spawn("cut -d':' -f1 | sort | uniq -c") - Count by hour`

	u.Subsections["error_analysis"] = `ERROR CLASSIFICATION:
spawn("grep -E '(ERROR|FATAL|CRITICAL)'") - High priority errors
spawn("grep -E '(WARN|WARNING)'") - Warnings
spawn("grep -v -E '(INFO|DEBUG|TRACE)'") - Non-informational

ERROR FREQUENCY:
spawn("grep ERROR | cut -d' ' -f3 | sort | uniq -c") - Error type frequency
spawn("grep ERROR | cut -d' ' -f1 | sort | uniq -c") - Error by time
spawn("grep FATAL | wc -l") - Fatal error count

ERROR CONTEXT:
spawn("grep -A10 -B5 'FATAL'") - Context around fatal errors
spawn("grep -A3 'Exception'") - Exception details`

	u.Subsections["pattern_detection"] = `ANOMALY DETECTION:
spawn("sort | uniq -c | sort -nr | head -10") - Most frequent items
spawn("grep -E '^.{100,}'") - Unusually long lines

LOG PATTERNS:
spawn("grep -E '\\[(ERROR|WARN)\\]'") - Bracketed log levels
spawn("cut -d' ' -f1-3") - Extract timestamp portion
spawn("grep -o '^[^:]*'") - Extract prefix before colon`

	u.Subsections["user_interaction"] = `PROGRESS REPORTING:
write(2, "Processing file 1/10...\\n")
write(2, "Progress: 50% complete\\n")
write(2, "Processed 1000 lines\\n")

STATUS UPDATES:
write(2, "Step 1: Reading input files\\n")
write(2, "Step 2: Analyzing data\\n")
write(2, "Step 3: Generating report\\n")

ERROR REPORTING:
write(2, "Warning: Empty file detected\\n")
write(2, "Error: Invalid format in line 42\\n")
write(2, "Info: Using default configuration\\n")`

	u.Subsections["conditional_branching"] = `DATA-DRIVEN BRANCHING:
if input contains "CSV":
  process_as_csv()
elif input contains "JSON":
  process_as_json()
else:
  process_as_text()

SIZE-BASED PROCESSING:
lines = spawn("wc -l")
if lines > 10000:
  write(2, "Large file detected, using streaming\\n")
  process_in_chunks()
else:
  process_all_at_once()`

	u.Subsections["progress_tracking"] = `STEP COUNTER:
total_steps = 5
for step in range(1, total_steps + 1):
  write(2, f"Step {step}/{total_steps}: {step_description}\\n")
  execute_step(step)

FILE COUNTER:
file_count = len(input_files)
for i, file in enumerate(input_files):
  write(2, f"Processing file {i+1}/{file_count}: {file}\\n")
  process_file(file)`

	u.Subsections["virtual_file_debug"] = `FILE STATUS CHECK:
write(2, "Checking virtual file status\\n")
try:
  test_fd = open("target_file", "r")
  write(2, "File exists and readable\\n")
  close(test_fd)
except:
  write(2, "File not found or consumed\\n")

RECREATION PATTERN:
open("temp_data", "w") → new_fd
write(new_fd, original_data)
close(new_fd)
write(2, "Virtual file recreated\\n")`

	u.Subsections["advanced_patterns"] = `MULTI-STAGE PIPELINE:
# Stage 1: Clean data
clean_fd = spawn("sed 's/[^a-zA-Z0-9 ]//g'", input_fd)
# Stage 2: Normalize
norm_fd = spawn("tr '[:upper:]' '[:lower:]'", clean_fd)
# Stage 3: Analyze
result = spawn("sort | uniq -c | sort -nr", norm_fd)

CONDITIONAL PIPELINE:
data_type = detect_data_type(input)
if data_type == "log":
  result = spawn("grep ERROR | cut -d' ' -f1-3")
elif data_type == "csv":
  result = spawn("tail -n +2 | cut -d',' -f1,3")`

	u.Subsections["first_steps"] = `HELLO WORLD:
1. read(0) - Read some input
2. write(1, "Hello: " + input) - Add greeting
3. exit(0) - Success exit

SIMPLE COUNTING:
1. data = read(0) - Read input
2. count = spawn("wc -l") - Count lines
3. write(1, "Lines: " + count) - Output count
4. exit(0)

BASIC FILTERING:
1. content = read(0) - Read input  
2. filtered = spawn("grep pattern") - Filter data
3. write(1, filtered) - Output filtered data
4. exit(0)`

	u.Subsections["learning_progression"] = `LEVEL 1 - BASIC I/O:
- read() and write() operations
- Understanding file descriptors
- Simple exit() usage

LEVEL 2 - SIMPLE COMMANDS:
- spawn("cat") - Display data
- spawn("wc") - Count operations
- spawn("head"), spawn("tail") - Data sampling

LEVEL 3 - FILTERING:
- spawn("grep pattern") - Pattern matching
- spawn("sort") - Data ordering
- spawn("uniq") - Duplicate removal

LEVEL 4 - PIPELINES:
- spawn("grep pattern | wc -l") - Combined operations
- spawn("sort | uniq -c") - Multi-stage processing
- Understanding pipeline efficiency

LEVEL 5 - VIRTUAL FILES:
- open() and close() operations
- Multi-stage processing with temp files
- Understanding PIPE behavior

LEVEL 6 - COMPLEX WORKFLOWS:
- Error handling patterns
- Conditional processing
- Multi-file operations`
}

// initKeyMappings initializes the mapping from keys to subsections
func (u *UsageData) initKeyMappings() {
	u.KeyMappings["data_analysis"] = []string{
		"basic_workflow", "pipeline_patterns", "statistical_commands", "filtering",
	}

	u.KeyMappings["text_processing"] = []string{
		"basic_workflow", "string_transformation", "virtual_file_usage", "advanced_patterns",
	}

	u.KeyMappings["file_operations"] = []string{
		"basic_workflow", "multi_file_handling", "virtual_file_usage", "comparison_diff",
	}

	u.KeyMappings["content_search"] = []string{
		"basic_workflow", "pattern_matching", "extraction_patterns", "complex_search",
	}

	u.KeyMappings["format_conversion"] = []string{
		"basic_workflow", "structured_data", "encoding_conversion", "error_handling",
	}

	u.KeyMappings["log_analysis"] = []string{
		"basic_workflow", "error_analysis", "time_series", "pattern_detection",
	}

	u.KeyMappings["batch_processing"] = []string{
		"basic_workflow", "multi_file_handling", "error_handling", "progress_tracking",
	}

	u.KeyMappings["interactive_workflow"] = []string{
		"basic_workflow", "user_interaction", "conditional_branching", "progress_tracking",
	}

	u.KeyMappings["debugging"] = []string{
		"debug_techniques", "error_handling", "virtual_file_debug", "common_errors",
	}

	u.KeyMappings["basic_operations"] = []string{
		"basic_workflow", "fd_concepts", "learning_progression", "first_steps",
	}

	u.KeyMappings["command_usage"] = []string{
		"tool_reference", "spawn_commands", "pipeline_patterns", "advanced_patterns",
	}
}

// GetHelp implements the help command
func GetHelp(args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("no keys provided")
	}

	// Validate keys
	usageData := NewUsageData()
	for _, key := range args {
		if _, exists := usageData.KeyMappings[key]; !exists {
			return fmt.Errorf("invalid key: %s", key)
		}
	}

	// Collect all subsections from requested keys
	subsectionSet := make(map[string]bool)
	for _, key := range args {
		for _, subsection := range usageData.KeyMappings[key] {
			subsectionSet[subsection] = true
		}
	}

	// Convert to sorted slice for consistent output
	var subsections []string
	for subsection := range subsectionSet {
		subsections = append(subsections, subsection)
	}
	sort.Strings(subsections)

	// Build response
	fmt.Fprintf(stdout, "USAGE INFORMATION FOR: %s\n\n", strings.Join(args, ", "))

	for i, subsection := range subsections {
		if i > 0 {
			fmt.Fprint(stdout, "\n")
		}
		fmt.Fprintf(stdout, "=== %s ===\n", strings.ToUpper(subsection))
		if content, exists := usageData.Subsections[subsection]; exists {
			fmt.Fprint(stdout, content)
			fmt.Fprint(stdout, "\n")
		} else {
			fmt.Fprint(stdout, "Content not available\n")
		}
	}

	return nil
}
