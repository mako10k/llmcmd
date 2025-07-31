# System Prompt Redesign Plan

## 現状の問題
1. システムプロンプトが長大すぎる（トークンコストが高い）
2. 固定化したいがプロンプトキャッシュを活用できていない
3. 詳細な使用方法がシステムプロンプトに埋め込まれている

## 新しい設計方針

### 1. 短縮システムプロンプト
システムプロンプトは最小限の情報のみ含む：
- llmcmd/llmsh の基本的な役割
- 利用可能ツールのリスト（詳細な説明なし）
- get_usage ツールで詳細情報を取得する指示

### 2. get_usages ツールの実装
- 用途別の詳細な使用方法を動的に取得
- 複数キー指定で必要な情報のみ取得
- サブセクション構造で重複排除とトークン最適化

## get_usages ツールのキー設計

### サブセクション構造の設計思想
各キーは複数のサブセクションを持ち、複数キー指定時は重複するサブセクションを統合。
効率的な情報提供と重複排除によるトークン最適化を実現。

### 利用可能キー一覧

#### 1. **"data_analysis"** - Data analysis and aggregation
- **Subsections**: ["basic_workflow", "pipeline_patterns", "statistical_commands", "filtering"]
- **Target Commands**: grep, sort, uniq, wc, head, tail, cut combinations

#### 2. **"text_processing"** - Text transformation and formatting
- **Subsections**: ["basic_workflow", "string_transformation", "virtual_file_usage", "advanced_patterns"]
- **Target Commands**: sed, tr, awk-style processing, regex

#### 3. **"file_operations"** - File operations and management
- **Subsections**: ["basic_workflow", "multi_file_handling", "virtual_file_usage", "comparison_diff"]
- **Target Commands**: cat, cp, mv, diff, find-like operations

#### 4. **"content_search"** - Content search and extraction
- **Subsections**: ["basic_workflow", "pattern_matching", "extraction_patterns", "complex_search"]
- **Target Commands**: grep, find, locate-style processing

#### 5. **"format_conversion"** - Format conversion
- **Subsections**: ["basic_workflow", "structured_data", "encoding_conversion", "error_handling"]
- **Target Commands**: CSV/JSON/XML conversion, encoding transformation

#### 6. **"log_analysis"** - Log analysis and monitoring
- **Subsections**: ["basic_workflow", "error_analysis", "time_series", "pattern_detection"]
- **Target Commands**: tail -f style, log parsing, statistics

#### 7. **"batch_processing"** - Batch processing and bulk operations
- **Subsections**: ["basic_workflow", "multi_file_handling", "error_handling", "progress_tracking"]
- **Target Commands**: Bulk file conversion and processing

#### 8. **"interactive_workflow"** - Interactive processing
- **Subsections**: ["basic_workflow", "user_interaction", "conditional_branching", "progress_tracking"]
- **Target Commands**: Prompt display, conditional processing

#### 9. **"debugging"** - Debugging and troubleshooting
- **Subsections**: ["debug_techniques", "error_handling", "virtual_file_debug", "common_errors"]

#### 10. **"basic_operations"** - Basic operations and learning
- **Subsections**: ["basic_workflow", "fd_concepts", "learning_progression", "first_steps"]

#### 11. **"command_usage"** - Detailed command usage
- **Subsections**: ["tool_reference", "spawn_commands", "pipeline_patterns", "advanced_patterns"]

### Multiple key support
Combine multiple keys to get integrated subsections with deduplication:
- `get_usages(["data_analysis", "text_processing"])` - Both subsections with deduplication
- `get_usages(["command_usage", "debugging"])` - Command details and debugging info
- `get_usages(["basic_operations"])` - Basic operations only

### Subsection detailed definitions

#### **basic_workflow** - Basic workflow patterns
```
MINIMAL PATTERN:
1. read(fd) - Read data
2. [process] - Process data
3. write(1, result) - Output result
4. exit(0) - Normal termination

STANDARD WORKFLOW:
1. read(fd) - Get input data
2. spawn("command") - Execute processing
3. read(out_fd) - Get processing result
4. write(1, processed_data) - Final output
5. exit(0) - Complete
```

#### **fd_concepts** - File descriptor concepts
```
FD MAPPING:
- fd=0: stdin (standard input)
- fd=1: stdout (result output)
- fd=2: stderr (error/debug output)
- fd=3+: input files ($1, $2, $3...)

USAGE PATTERNS:
- read(0): Read from standard input
- read(3): Read from first input file
- write(1, data): Output result to stdout
- write(2, "debug info"): Debug info to stderr
```

#### **tool_reference** - Tool detailed reference
```
read(fd, [lines], [count]) - Read data
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
exit(code) - Terminate program (0=success, 1=error)
```

#### **spawn_commands** - Spawn executable commands
```
TEXT PROCESSING:
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
- spawn("tr '[:upper:]' '[:lower:]'"): Convert uppercase to lowercase
```

#### **pipeline_patterns** - Pipeline processing patterns
```
FILTERING PIPELINE:
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
→ Extract error lines → Count lines
```

#### **virtual_file_usage** - Virtual file operations
```
CREATE AND WRITE:
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
3. read(stage2_fd) → processed_data
```

#### **multi_file_handling** - 複数ファイル処理
```
SEQUENTIAL PROCESSING:
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
process_combined(all_data)
```

#### **error_handling** - Error handling
```
GRACEFUL CONTINUATION:
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
  exit(1)
```

#### **pattern_matching** - Pattern matching
```
BASIC PATTERNS:
spawn("grep 'pattern'") - Simple string
spawn("grep -E 'pattern1|pattern2'") - OR search
spawn("grep -v 'exclude'") - Exclude search

CONTEXT SEARCH:
spawn("grep -B3 -A3 'pattern'") - 3 lines before/after
spawn("grep -n 'pattern'") - With line numbers
spawn("grep -c 'pattern'") - Count only

FIELD-BASED MATCHING:
spawn("cut -d',' -f2 | grep 'pattern'") - Specific field search
spawn("grep '^ERROR' | cut -d' ' -f2-") - Line start pattern + field extract
```

#### **string_transformation** - String transformation
```
SUBSTITUTION:
spawn("sed 's/old/new/g'") - Global replace
spawn("sed 's/^/prefix_/'") - Add to line start
spawn("sed 's/$/suffix/'") - Add to line end

CHARACTER CONVERSION:
spawn("tr '[:upper:]' '[:lower:]'") - Uppercase to lowercase
spawn("tr -d '\r'") - Delete characters
spawn("tr ' ' '_'") - Character replacement

ADVANCED TRANSFORMATION:
spawn("sed -n '/start/,/end/p'") - Range extraction
spawn("sed '/pattern/d'") - Delete pattern lines
```

#### **comparison_diff** - Comparison and diff
```
FILE COMPARISON:
1. open("file1_temp", "w"), write(fd, file1_data)
2. open("file2_temp", "w"), write(fd, file2_data)
3. spawn("diff file1_temp file2_temp")

LINE-BY-LINE DIFF:
spawn("diff -u file1 file2") - Unified diff format
spawn("diff --side-by-side file1 file2") - Side-by-side display

CONTENT ANALYSIS:
spawn("comm -12 file1 file2") - Common lines
spawn("comm -23 file1 file2") - Lines only in file1
```

#### **common_errors** - Common errors
```
EMPTY OUTPUT:
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
Solution: Use only built-in commands
```

#### **debug_techniques** - Debug techniques
```
OUTPUT VERIFICATION:
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
3. Check if content matches expected
```

#### **statistical_commands** - Statistical commands
```
COUNTING:
spawn("wc -l") - Line count
spawn("wc -w") - Word count
spawn("wc -c") - Character count

FREQUENCY ANALYSIS:
spawn("sort | uniq -c") - Count occurrences
spawn("sort | uniq -c | sort -nr") - Sort by frequency
spawn("cut -d' ' -f1 | sort | uniq -c") - Field frequency

SAMPLING:
spawn("head -100") - First 100 lines
spawn("tail -100") - Last 100 lines
spawn("shuf -n 10") - Random 10 lines (if available)
```

#### **filtering** - Data filtering
```
BASIC FILTERING:
spawn("grep pattern") - Include matching lines
spawn("grep -v pattern") - Exclude matching lines
spawn("grep -i pattern") - Case-insensitive match

RANGE FILTERING:
spawn("sed -n '10,20p'") - Lines 10-20
spawn("head -n 50 | tail -n 10") - Lines 41-50
spawn("awk 'NR>=10 && NR<=20'") - Line range (if awk available)

CONDITIONAL FILTERING:
spawn("grep '^[0-9]'") - Lines starting with numbers
spawn("grep '[A-Z]'") - Lines containing uppercase
spawn("grep -E '^(ERROR|WARN)'") - Multiple patterns
```

#### **extraction_patterns** - Data extraction patterns
```
FIELD EXTRACTION:
spawn("cut -d',' -f1,3") - CSV columns 1 and 3
spawn("cut -d' ' -f2-") - All fields from 2nd onward
spawn("cut -c10-20") - Characters 10-20

PATTERN EXTRACTION:
spawn("grep -o '[0-9]+\\.[0-9]+\\.[0-9]+\\.[0-9]+'") - IP addresses
spawn("grep -o '[a-zA-Z0-9._-]+@[a-zA-Z0-9._-]+'") - Email addresses
spawn("sed -n 's/.*\\([0-9]\\{4\\}-[0-9]\\{2\\}-[0-9]\\{2\\}\\).*/\\1/p'") - Dates

STRUCTURED EXTRACTION:
spawn("sed -n '/START/,/END/p'") - Extract between markers
spawn("grep -A5 -B5 'pattern'") - Context extraction
spawn("awk '/start_pattern/,/end_pattern/'") - Range extraction
```

#### **complex_search** - Complex search operations
```
MULTI-PATTERN SEARCH:
spawn("grep -E 'pattern1|pattern2|pattern3'") - Multiple patterns
spawn("grep pattern1 | grep pattern2") - AND search
spawn("grep -f patterns.txt") - Patterns from file

RECURSIVE PATTERNS:
spawn("grep -r 'pattern' directory") - Recursive search (if supported)
spawn("find . -name '*.txt' -exec grep 'pattern' {} \\;") - Find and grep

ADVANCED MATCHING:
spawn("grep -P '\\d{3}-\\d{3}-\\d{4}'") - Perl regex (if supported)
spawn("grep -o '\\w\\+' | sort | uniq") - Extract words
spawn("grep -v '^#' | grep -v '^$'") - Skip comments and empty lines
```

#### **structured_data** - Structured data processing
```
CSV PROCESSING:
spawn("cut -d',' -f1") - First column
spawn("tail -n +2") - Skip header
spawn("sed 's/,/\\t/g'") - CSV to TSV
spawn("awk -F',' '{print $1,$3}'") - Specific columns

JSON-LIKE PROCESSING:
spawn("grep -o '\"[^\"]*\":[^,}]*'") - Extract key-value pairs
spawn("sed -n 's/.*\"key\":\"\\([^\"]*\\)\".*/\\1/p'") - Extract specific value
spawn("tr -d ' \\t' | grep -o '{[^}]*}'") - Extract objects

XML-LIKE PROCESSING:
spawn("grep -o '<tag>[^<]*</tag>'") - Extract tag content
spawn("sed -n 's/.*<tag>\\([^<]*\\)<\\/tag>.*/\\1/p'") - Tag value
spawn("grep -P '(?<=<tag>)[^<]+(?=</tag>)'") - Content between tags
```

#### **encoding_conversion** - Encoding and format conversion
```
CHARACTER ENCODING:
spawn("iconv -f UTF-8 -t ASCII//IGNORE") - Encoding conversion
spawn("tr -d '\\r'") - Remove carriage returns
spawn("dos2unix") - Convert line endings (if available)

CASE CONVERSION:
spawn("tr '[:upper:]' '[:lower:]'") - Lowercase
spawn("tr '[:lower:]' '[:upper:]'") - Uppercase
spawn("sed 's/\\b\\w/\\U&/g'") - Title case (first letter of words)

FORMAT NORMALIZATION:
spawn("sed 's/[[:space:]]\\+/ /g'") - Normalize whitespace
spawn("sed 's/^[[:space:]]*//'") - Remove leading whitespace
spawn("sed 's/[[:space:]]*$//'") - Remove trailing whitespace
```

#### **time_series** - Time series data processing
```
DATE FILTERING:
spawn("grep '2024-01'") - Filter by year-month
spawn("grep '^[0-9]\\{4\\}-[0-9]\\{2\\}-[0-9]\\{2\\}'") - Date pattern
spawn("awk '$1 >= \"2024-01-01\" && $1 <= \"2024-01-31\"'") - Date range

TIME EXTRACTION:
spawn("cut -d' ' -f1") - Extract date field
spawn("sed 's/.*\\([0-9]\\{2\\}:[0-9]\\{2\\}:[0-9]\\{2\\}\\).*/\\1/'") - Extract time
spawn("cut -d'T' -f1") - Extract date from ISO format

AGGREGATION BY TIME:
spawn("cut -d' ' -f1 | sort | uniq -c") - Count by date
spawn("cut -d':' -f1 | sort | uniq -c") - Count by hour
spawn("grep '$(date +%Y-%m-%d)'") - Today's entries
```

#### **error_analysis** - Error analysis patterns
```
ERROR CLASSIFICATION:
spawn("grep -E '(ERROR|FATAL|CRITICAL)'") - High priority errors
spawn("grep -E '(WARN|WARNING)'") - Warnings
spawn("grep -v -E '(INFO|DEBUG|TRACE)'") - Non-informational

ERROR FREQUENCY:
spawn("grep ERROR | cut -d' ' -f3 | sort | uniq -c") - Error type frequency
spawn("grep ERROR | cut -d' ' -f1 | sort | uniq -c") - Error by time
spawn("grep FATAL | wc -l") - Fatal error count

ERROR CONTEXT:
spawn("grep -A10 -B5 'FATAL'") - Context around fatal errors
spawn("grep -A3 'Exception'") - Exception details
spawn("sed -n '/ERROR/,/^$/p'") - Error blocks
```

#### **pattern_detection** - Pattern detection
```
ANOMALY DETECTION:
spawn("sort | uniq -c | sort -nr | head -10") - Most frequent items
spawn("awk '{count[NF]++} END {for(i in count) print i, count[i]}'") - Field count distribution
spawn("grep -E '^.{100,}'") - Unusually long lines

SEQUENCE PATTERNS:
spawn("grep -E '^[0-9]+\\s+[0-9]+\\s+[0-9]+$'") - Number sequences
spawn("awk 'prev && ($1 - prev) > 100 {print prev, $1}; {prev=$1}'") - Gaps in sequences
spawn("sort -n | awk 'NR>1 && $1==prev {print \"Duplicate:\", $1}; {prev=$1}'") - Duplicates

LOG PATTERNS:
spawn("grep -E '\\[(ERROR|WARN)\\]'") - Bracketed log levels
spawn("cut -d' ' -f1-3") - Extract timestamp portion
spawn("grep -o '^[^:]*'") - Extract prefix before colon
```

#### **user_interaction** - User interaction simulation
```
PROGRESS REPORTING:
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
write(2, "Info: Using default configuration\\n")
```

#### **conditional_branching** - Conditional processing
```
DATA-DRIVEN BRANCHING:
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
  process_all_at_once()

FORMAT DETECTION:
first_line = read(fd, 1)
if first_line.startswith("#"):
  write(2, "Comment format detected\\n")
  process_with_comments()
```

#### **progress_tracking** - Progress tracking
```
STEP COUNTER:
total_steps = 5
for step in range(1, total_steps + 1):
  write(2, f"Step {step}/{total_steps}: {step_description}\\n")
  execute_step(step)
  write(2, f"Completed {step}/{total_steps}\\n")

FILE COUNTER:
file_count = len(input_files)
for i, file in enumerate(input_files):
  write(2, f"Processing file {i+1}/{file_count}: {file}\\n")
  process_file(file)

PERCENTAGE PROGRESS:
processed = 0
total = get_total_items()
for item in items:
  process_item(item)
  processed += 1
  percent = (processed * 100) // total
  write(2, f"Progress: {percent}%\\n")
```

#### **virtual_file_debug** - Virtual file debugging
```
FILE STATUS CHECK:
# Before reading
write(2, "Checking virtual file status\\n")
try:
  test_fd = open("target_file", "r")
  write(2, "File exists and readable\\n")
  close(test_fd)
except:
  write(2, "File not found or consumed\\n")

RECREATION PATTERN:
# If file was consumed, recreate it
open("temp_data", "w") → new_fd
write(new_fd, original_data)
close(new_fd)
write(2, "Virtual file recreated\\n")

DEBUG CONTENT:
# Verify file content before processing
debug_fd = open("debug_copy", "w")
write(debug_fd, test_data)
close(debug_fd)
verify_fd = open("debug_copy", "r")
content = read(verify_fd)
write(2, "Content length: " + len(content) + "\\n")
```

#### **advanced_patterns** - Advanced processing patterns
```
MULTI-STAGE PIPELINE:
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
  result = spawn("tail -n +2 | cut -d',' -f1,3")

PARALLEL PROCESSING SIMULATION:
# Process multiple files conceptually in parallel
results = []
for fd in [3, 4, 5]:
  result = spawn("wc -l", fd)
  results.append(result)
combine_results(results)
```

#### **first_steps** - First steps for beginners
```
HELLO WORLD:
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
4. exit(0)
```

#### **learning_progression** - Learning progression
```
LEVEL 1 - BASIC I/O:
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
- Multi-file operations
```
## 短縮システムプロンプト案

```
You are llmcmd, a text processing assistant within the llmsh shell environment.

🏠 ENVIRONMENT: llmsh (LLM-powered shell)
🔧 TOOLS: read, write, open, spawn, close, exit, get_usages
🛠️ COMMANDS: Built-in text processing (cat, grep, sed, head, tail, sort, wc, tr)

📖 USAGE GUIDE:
Use get_usages(keys) for detailed information:
- data_analysis: Data analysis and aggregation
- text_processing: Text transformation and formatting  
- file_operations: File operations and management
- content_search: Content search and extraction
- format_conversion: Format conversion
- log_analysis: Log analysis and monitoring
- batch_processing: Batch processing and bulk operations
- interactive_workflow: Interactive processing
- debugging: Debugging and troubleshooting
- basic_operations: Basic operations and learning
- command_usage: Detailed command usage

Multiple keys supported: get_usages(["data_analysis", "debugging"])

🎯 WORKFLOW: read input → process → write output → exit
```

## Implementation specification

### get_usages tool definition
```json
{
  "name": "get_usages",
  "description": "Get detailed usage information for specific use cases",
  "parameters": {
    "type": "object",
    "properties": {
      "keys": {
        "type": "array",
        "items": {
          "type": "string",
          "enum": [
            "data_analysis", "text_processing", "file_operations",
            "content_search", "format_conversion", "log_analysis", 
            "batch_processing", "interactive_workflow", "debugging",
            "basic_operations", "command_usage"
          ]
        },
        "description": "List of usage keys to retrieve information for"
      }
    },
    "required": ["keys"]
  }
}
```

### Subsection integration logic
1. Collect all subsections from specified keys
2. Deduplicate overlapping subsections
3. Generate integrated subsection content
4. Add key-specific information

## Implementation steps

### Phase 1: get_usages tool implementation
1. Create get_usages.go in internal/tools/builtin/
2. Implement subsection structure as map format
3. Define key → subsection mappings
4. Implement deduplication and content integration logic
5. Integrate into tools/engine.go

### Phase 2: System prompt shortening
1. Replace system prompt in client.go with shortened version
2. Migrate detailed explanations to get_usages content
3. Add get_usages to OpenAI API tool definitions

### Phase 3: Testing and adjustment
1. Test each key and subsection combination
2. Verify deduplication logic operation
3. Measure and compare token usage
4. Adjust subsection structure as needed

### Data structure example
```go
type SubsectionContent map[string]string
type KeyMapping map[string][]string
type UsageData struct {
    Subsections SubsectionContent
    KeyMappings KeyMapping
}

// Example: "data_analysis" → ["basic_workflow", "pipeline_patterns", ...]
// Multiple keys: integrate all subsections with deduplication
```

## Expected benefits
- System prompt token cost reduction (estimated 70% reduction)
- Maximize prompt cache effectiveness
- Optimize cost by retrieving only necessary information dynamically
- More flexible information provision
