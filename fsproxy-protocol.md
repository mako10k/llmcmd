# FS Proxy Protocol Specification

## Revision Notice (JSON Framing Adoption)

This document has been updated to designate a JSON length‑prefixed framing (4‑byte big‑endian unsigned length + UTF‑8 JSON payload) as the primary protocol (Option A decision). The previously described line‑oriented textual command syntax (e.g. `OPEN filename mode is_top_level`) is now considered a Legacy Variant and retained in an appendix for reference / possible compatibility bridges.

Primary goals of the revision:
1. Unified structured messages (easier extension, consistent error payloads)
2. Explicit error code taxonomy (Fail‑First, machine readable)
3. Reservation of future operations (QUOTA, LLMCMD) at protocol level
4. Stable foundation for builtin shell commands implemented directly in-process (no exec) using a VFS client

---

## Overview

The FS Proxy Protocol is a communication protocol designed to control file access by child processes in LLM execution environments. The parent process (llmsh/llmcmd) acts as an FS Proxy Manager, handling file operation requests from child processes.

## Architecture

```
┌─────────────────┐    FD 3 (pipe)    ┌─────────────────┐
│   Child Process │ ◄──────────────► │  Parent Process │
│   (LLM Execution)│                  │   (llmsh/llmcmd)│
│                │                  │                │
│  FS Client     │                  │  FS Proxy      │
│                │                  │  Manager       │
└─────────────────┘                  └─────────────────┘
                                           │
                                           ▼
                                     ┌─────────────┐
                                     │     VFS     │
                                     │ (Restricted │
                                     │ Environment)│
                                     └─────────────┘
```

## Protocol Specification

### Communication Method (Primary / JSON)

- Transport: Inherited pipe or socket FD (commonly FD 3) parent <-> child
- Framing: 4 bytes (big-endian u32) length prefix followed by UTF-8 JSON document
- Encoding: All binary file contents are base64 within JSON for READ/WRITE (chunked)
- Synchronization: Strict request/response (one response per request id)
- Concurrency: Client may pipeline (send multiple requests) but must correlate via `id`

Reserved Handles (stdio):
- Handles 0, 1, 2 are reserved for stdin, stdout, stderr respectively. The VFS server (vfsd) MUST NOT allocate or accept these handles; any `read`/`write`/`close` on 0..2 MUST be rejected with `E_PERM` by the server if received.
- The parent-side MUX MUST intercept requests targeting 0..2 and handle them locally by reading from process stdin (fd 0) and writing to stdout/stderr (fd 1/2), and acknowledge `close` as a no-op. Such requests MUST NOT be forwarded upstream.

### Message Envelope

Request JSON:
```
{
    "id": "<string>",
    "op": "<operation>",
    "params": { ... }          // optional; object only
}
```

Response JSON:
```
// Success
{
    "id": "<same id>",
    "ok": true,
    "result": { ... }          // optional result object
}

// Error
{
    "id": "<same id>",
    "ok": false,
    "error": { "code": "E_NOENT", "message": "human readable" }
}
```

Error Codes (initial set):
| Code | Meaning |
|------|---------|
| E_ARG | Invalid / missing parameter |
| E_NOENT | No such file / handle / path |
| E_PERM | Permission / allowlist violation |
| E_IO | Underlying I/O failure |
| E_CLOSED | Handle already closed |
| E_UNSUPPORTED | Operation not supported in build |
| E_RANGE | Size or limit exceeded (e.g. max read) |

Maximum chunk size for `read` requests (`max` parameter) is 4096 bytes. `max == 0` MUST yield `E_ARG`.

### Reserved / Planned Operations

The following operations are reserved (placeholders) for higher-level integrations and must not be repurposed:
- `QUOTA` : Query aggregated resource/token quotas (successor of legacy `LLM_QUOTA`).
- `LLMCMD` : Execute an LLM command session (superseding legacy `LLM_CHAT`) within controlled VFS context.

Until implemented they MUST return `ok:false` with `E_UNSUPPORTED`.

### Legacy Text Protocol (Appendix Preview)
The earlier line‑oriented format remains documented in the Appendix (see *Legacy Textual Variant*). New clients SHOULD implement the JSON framing. Servers MAY optionally provide an auto‑detect shim (first byte not `{` => treat as legacy) but this is NOT required for minimum compliance.

---

### Message Format

#### Request Format

```
COMMAND param1 param2 ...\n
[binary_data]  # Only for WRITE command
```

#### Response Format

```
STATUS data\n
[binary_data]  # Only for READ command responses
```

- **STATUS**: `OK` or `ERROR`
- **data**: Status-dependent data or error message

## Command Specification

### 1. open (JSON Primary)

Opens (or creates) a file. Path policy (allowlist vs virtual temp) is enforced server side.

Request:
```
{ "id":"1", "op":"open", "params": { "path":"test.txt", "mode":"w" } }
```
Parameters:
- `path` (string, required)
- `mode` (string, required): `r` | `w` | `a` | `rw`

Notes:
- Response is intentionally minimal: ONLY the numeric `handle` is returned on success. No creation / virtual / capability metadata is included (future `stat` op may expose details if needed).
- `rw` mode: read/write; creates file (or virtual) if absent without truncation.
- `w` mode: write-only; truncate/create semantics.
- `a` mode: append-only (write); read attempts will yield `E_PERM`.
- `r` mode: read-only; requires existing allowlisted or previously created virtual path.

Success Response:
```
{ "id":"1", "ok":true, "result": { "handle": 42 } }
```

Error Response Example:
```
{ "id":"1", "ok":false, "error": { "code":"E_ARG", "message":"missing path" } }
```

### 2. read
Request:
```
{ "id":"2", "op":"read", "params": { "h":42, "max":4096 } }
```
Result (data base64, may be empty when eof=true):
```
{ "id":"2", "ok":true, "result": { "data":"aGVsbG8=", "eof":false } }
```
EOF example:
```
{ "id":"3", "ok":true, "result": { "data":"", "eof":true } }
```

### 3. write
Request:
```
{ "id":"4", "op":"write", "params": { "h":42, "data":"aGVsbG8=", "final":false } }
```
Response:
```
{ "id":"4", "ok":true, "result": { "written":5 } }
```
`final` (optional bool) may be used by clients in future for flush semantics (ignored now).

### 4. close
Request:
```
{ "id":"5", "op":"close", "params": { "h":42 } }
```
Response:
```
{ "id":"5", "ok":true }
```

**Error Patterns**:
- `ERROR CLOSE requires fileno` - Missing parameters
- `ERROR invalid fileno: abc` - Invalid file number
- `ERROR CLOSE not yet implemented` - Not implemented (Phase 1)

### 5. LLMCMD (Reserved; replaces legacy LLM_CHAT)

Status: RESERVED. Servers MUST return `E_UNSUPPORTED` until implemented.

Intended Purpose (preview): Execute higher-level LLM operations within the same controlled VFS / quota environment. Will subsume prior `LLM_CHAT` design. Final parameter schema TBD (will likely carry structured prompt, file handle references, and stream options).

### 6. QUOTA (Reserved; replaces legacy LLM_QUOTA)

Status: RESERVED. Servers MUST return `E_UNSUPPORTED` until implemented.

Intended Purpose: Provide aggregated and weighted token usage snapshot.

---

## Legacy Textual Variant (Appendix)

The following sections preserve the original line-based command forms for historical reference and potential transitional tooling. They are **NOT** the primary specification anymore.

### Legacy OPEN Command (Text)
```
OPEN filename mode is_top_level\n
```
... (original description retained above in earlier revision; keep or prune as the project deprecates legacy mode) ...

### Legacy READ / WRITE / CLOSE / LLM_CHAT / LLM_QUOTA
Refer to pre-revision copy. New development SHOULD ignore these unless building a compatibility bridge.

---

Executes OpenAI ChatCompletion API by forking child process and calling app.ExecuteInternal() function with VFS environment.

#### Request
```
LLM_CHAT is_top_level stdin_fd stdout_fd stderr_fd input_files_count prompt_length\n
[input_files_text]
[prompt_text]
```

**Parameters**:
- `is_top_level`: Top-level execution flag ("true" or "false")
- `stdin_fd`: File descriptor number for stdin input (pipeline source)
- `stdout_fd`: File descriptor number for stdout output (pipeline destination)
- `stderr_fd`: File descriptor number for stderr output (typically 2)
- `input_files_count`: Byte count of input files text
- `prompt_length`: Byte count of prompt text
- `input_files_text`: Input file paths separated by newlines (VFS file paths)
- `prompt_text`: User instruction text

**Implementation Strategy**:
- **Fork + Function Call**: Forks child process and calls `app.ExecuteInternal()` function directly (no binary execution)
- **Pipeline Support**: Maps specified stdin_fd/stdout_fd/stderr_fd to child process streams
- **VFS Environment**: VFS Proxy Pipe (FD3) inheritance for file operations
- **Existing Logic Reuse**: CreateInitialMessages operates within VFS constraints automatically
- **Token Management**: Existing quota calculation and readFileWithTokenLimit apply automatically
- **Security**: No external binary execution - internal function calls only

**Model Selection Logic (existing llmcmd pattern)**:
- `is_top_level=true`: Uses `config.model` (user-specified model)
- `is_top_level=false`: Uses `config.internalModel` (typically "gpt-4o-mini")

**Automatic VFS Integration**:
- **FD Stream Mapping**: stdin_fd/stdout_fd/stderr_fd streams mapped to subprocess pipes
- **File Information**: getStdFileInfo() operates via VFS within subprocess  
- **Pipeline Compatibility**: Supports non-standard FD assignments in llmsh pipelines
- **Token Limits**: MIN(standard_limit, quota_calculated_limit) applied automatically

#### Response

**Success**:
```
OK response_size quota_status\n
[response_json]
```
- `response_size`: Byte count of response JSON
- `quota_status`: Quota usage status (e.g., "1250.5/5000 weighted tokens")
- `response_json`: ChatCompletion response JSON with existing token tracking structure

**Error**:
```
ERROR message\n
```

**Error Patterns**:
- `ERROR LLM_CHAT requires is_top_level, stdin_fd, stdout_fd, stderr_fd, input_files_count, and prompt_length` - Missing parameters
- `ERROR invalid is_top_level: maybe` - Invalid top-level flag
- `ERROR invalid stdin_fd: -1` - Invalid stdin file descriptor
- `ERROR invalid stdout_fd: -1` - Invalid stdout file descriptor  
- `ERROR invalid stderr_fd: -1` - Invalid stderr file descriptor
- `ERROR fd not found: 5` - Referenced FD does not exist in VFS table
- `ERROR quota exceeded: cannot make LLM call` - Quota exceeded
- `ERROR subprocess execution failed: reason` - Fork+ExecuteInternal execution error
- `ERROR OpenAI API call failed: reason` - API call error (from subprocess)
- `ERROR LLM not available` - LLM functionality unavailable
- `ERROR failed to read input files data` - Input files data read error
- `ERROR failed to read prompt data` - Prompt data read error

#### Examples

```
# Success case (top-level execution with file input)
Client → Server: "LLM_CHAT true 0 1 2 25 45\n/tmp/input.txt\n/tmp/output.txt\nAnalyze the input data and create a summary."
Server → Client: "OK 156 1250.5/5000 weighted tokens\n{\"choices\":[{\"message\":{\"content\":\"Analysis complete...\"}}],\"usage\":{\"prompt_tokens\":15,\"completion_tokens\":8}}"

# Success case (child process execution)
Client → Server: "LLM_CHAT false 0 1 2 0 20\n\nExecute simple analysis"
Server → Client: "OK 142 1350.0/5000 weighted tokens\n{\"choices\":[{\"message\":{\"content\":\"Simple task completed\"}}],\"usage\":{\"prompt_tokens\":8,\"completion_tokens\":6}}"

# Error case (invalid is_top_level)
Client → Server: "LLM_CHAT invalid 0 1 2 0 0\n"
Server → Client: "ERROR invalid is_top_level: invalid\n"

# Error case (invalid FD)
Client → Server: "LLM_CHAT true -1 1 2 0 20\n\nTest prompt"
Server → Client: "ERROR invalid stdin_fd: -1\n"
```

<!-- Legacy quota section removed in favor of reserved QUOTA operation -->

## Error Handling

### Communication Level Errors

#### 1. Pipe Disconnection
```go
if err == io.EOF {
    // Child process closed the pipe (normal termination)
    return nil
}
```

#### 2. Read Errors
```go
log.Printf("FS Proxy: Error reading request: %v", err)
continue  // Log error and continue
```

#### 3. Send Errors
```go
log.Printf("FS Proxy: Error sending response: %v", err)
return err  // Fatal error - terminate
```

### Protocol Level Errors

#### 1. Empty Request
```
ERROR empty request
```

#### 2. Unknown Command
```
ERROR unknown command: INVALID
```

#### 3. Parameter Errors
```
ERROR OPEN requires filename and mode
ERROR invalid fileno: abc
ERROR invalid size: xyz
```

## Implementation Status

### Phase 1 (Completed)
- ✅ Basic protocol structure
- ✅ OPEN command (basic implementation)
- ✅ Error handling
- ✅ Communication foundation

### Phase 2 (Planned)
- ⏳ Complete fd management table
- ⏳ Full READ/WRITE/CLOSE implementation
- ⏳ llmsh integration
- ⏳ Pipeline support

### Phase 3 (New: VFS-Centralized LLM Execution)
- 🆕 LLM_CHAT command implementation
- 🆕 LLM_QUOTA command implementation
- 🆕 OpenAI API integration in VFS server
- 🆕 Unified quota management system
- 🆕 Unified LLM execution for llmcmd/llmsh

### Phase 4 (New: Resource Management)
- 🆕 Automatic file descriptor cleanup
- 🆕 Resource recovery on PIPE EOF detection
- 🆕 Process monitoring for abnormal termination handling
- 🆕 Client and file management tables
- 🆕 Resource propagation in hierarchical VFS

## Resource Management

### Automatic File Descriptor Cleanup

#### 1. VFS Server Side (FS Proxy Manager)
```go
// Automatic cleanup on PIPE EOF detection
func (proxy *FSProxyManager) handlePipeEOF(clientID string) {
    // Get all filenos opened by the client
    openFiles := proxy.getOpenFilesByClient(clientID)
    
    for _, fileno := range openFiles {
        // Auto-close files in VFS
        proxy.vfs.CloseFile(fileno)
        log.Printf("Auto-closed fileno %d for client %s (pipe EOF)", fileno, clientID)
    }
    
    // Cleanup client information
    proxy.removeClient(clientID)
}
```

#### 2. VFS Proxy Side (Intermediate Layer)
```go
// Notification to upstream on PIPE EOF detection
func (vfsProxy *VFSProxy) handlePipeEOF(downstreamClientID string) {
    // Get all filenos opened by downstream client
    openFiles := vfsProxy.getOpenFilesByDownstream(downstreamClientID)
    
    for _, fileno := range openFiles {
        // Send CLOSE request to upstream VFS server
        vfsProxy.sendCloseRequest(fileno)
        log.Printf("Sent close request for fileno %d (downstream EOF)", fileno)
    }
    
    // Cleanup downstream client information
    vfsProxy.removeDownstreamClient(downstreamClientID)
}
```

### Resource Tracking Tables

#### File Descriptor Management Table
```go
type FileDescriptorTable struct {
    mu    sync.RWMutex
    files map[int]*OpenFile // fileno -> OpenFile
}

type OpenFile struct {
    FileNo     int       `json:"fileno"`
    Filename   string    `json:"filename"`
    Mode       string    `json:"mode"`
    ClientID   string    `json:"client_id"`
    OpenedAt   time.Time `json:"opened_at"`
    IsTopLevel bool      `json:"is_top_level"`
}
```

#### Client Management Table
```go
type ClientTable struct {
    mu      sync.RWMutex
    clients map[string]*Client // clientID -> Client
}

type Client struct {
    ID          string    `json:"id"`
    PipeID      string    `json:"pipe_id"`
    OpenFiles   []int     `json:"open_files"`   // List of open filenos
    ConnectedAt time.Time `json:"connected_at"`
}
```

### Automatic Cleanup Triggers

#### 1. PIPE EOF Detection
```go
func (proxy *FSProxyManager) monitorPipe(pipe io.ReadWriter, clientID string) {
    defer func() {
        // Automatic cleanup on PIPE disconnection
        proxy.handlePipeEOF(clientID)
    }()
    
    for {
        request, err := proxy.readRequest(pipe)
        if err == io.EOF {
            log.Printf("Client %s disconnected (EOF)", clientID)
            break
        }
        if err != nil {
            log.Printf("Client %s error: %v", clientID, err)
            break
        }
        
        proxy.handleRequest(request, clientID)
    }
}
```

#### 2. Process Termination Detection
```go
func (proxy *FSProxyManager) monitorProcess(pid int, clientID string) {
    go func() {
        // Monitor process termination
        process, _ := os.FindProcess(pid)
        process.Wait()
        
        log.Printf("Process %d terminated, cleaning up client %s", pid, clientID)
        proxy.handlePipeEOF(clientID)
    }()
}
```

## Security Considerations

1. **VFS Restrictions**: Child processes can only access files through parent process VFS
2. **FD Management**: Parent process manages all file descriptors
3. **Parameter Validation**: All request parameters are validated
4. **Error Isolation**: Child process errors do not affect parent process design
5. **LLM Execution Control**: All OpenAI API calls centrally managed by VFS server
6. **Quota Management**: Real-time token usage monitoring and limits
7. **API Authentication**: OpenAI API keys managed only by VFS server
8. **Response Validation**: LLM API response validity verification
9. **Resource Leak Prevention**: Automatic file descriptor cleanup for resource protection
10. **Process Monitoring**: Reliable resource recovery on abnormal termination
11. **LLM Context Access Control**: LLM_QUOTA restricted to LLM execution contexts only

## Usage Examples

### Traditional File Operations

```go
// Child process side (FS Client)
client, _ := fsclient.NewFSClient()

// Open file (top-level execution)
fileno, _ := client.Open("data.txt", "w", true)

// Write data
data := []byte("Hello, World!")
client.Write(fileno, data)

// Close file
client.Close(fileno)
```

### New Feature: LLM Execution

```go
// Child process side (FS Client) - Top-level execution
client, _ := fsclient.NewFSClient()

// Check quota
quota, _ := client.LLMQuota()
fmt.Printf("Quota: %s\n", quota)

// LLM execution (top-level - prompt specified)
inputFiles := "/tmp/prompt.txt\n/tmp/data.csv"
prompt := "Analyze the input data and generate a summary report."
response, _ := client.LLMChat(true, inputFiles, prompt)
fmt.Printf("Response: %s\n", response.Choices[0].Message.Content)

// LLM execution (child process - restricted mode)
response2, _ := client.LLMChat(false, "", "Simple calculation: 2+2")
fmt.Printf("Response: %s\n", response2.Choices[0].Message.Content)

// Check updated quota
newQuota, _ := client.LLMQuota()
fmt.Printf("Updated quota: %s\n", newQuota)
```

```go
// Parent process side (FS Proxy Manager)
proxy := NewFSProxyManager(vfs, pipe, true)
proxy.SetLLMClient(openaiClient)  // Set OpenAI client

// Resource management configuration
proxy.EnableAutoCleanup(true)

go proxy.HandleFSRequest()  // Process in background
```

### Resource Management Usage Examples

```go
// Abnormal termination simulation
func simulateAbnormalTermination() {
    client, _ := fsclient.NewFSClient()
    
    // Open files
    fileno1, _ := client.Open("temp1.txt", "w", true)
    fileno2, _ := client.Open("temp2.txt", "w", true) 
    
    // Write data
    client.Write(fileno1, []byte("data1"))
    client.Write(fileno2, []byte("data2"))
    
    // Exit without CLOSE (simulate abnormal termination)
    os.Exit(1)  // Automatic cleanup will work on VFS server side
}

// Normal termination
func normalTermination() {
    client, _ := fsclient.NewFSClient()
    
    fileno, _ := client.Open("temp.txt", "w", true)
    client.Write(fileno, []byte("data"))
    client.Close(fileno)  // Explicit close
}
```
