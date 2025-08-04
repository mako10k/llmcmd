# FS Proxy Protocol Specification

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

### Communication Method

- **Transport**: Unix pipe (os.Pipe())
- **Inheritance**: Child processes access FS Proxy via FD 3
- **Data Format**: Text-based + Binary data
- **Synchronization**: Request/Response synchronous communication

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

### 1. OPEN Command

Opens a file and returns a file number (fileno).

#### Request
```
OPEN filename mode is_top_level\n
```

**Parameters**:
- `filename`: File name to open
- `mode`: File open mode
- `is_top_level`: Top-level execution flag ("true" or "false")

**Supported Modes**:
- `r`: Read-only (O_RDONLY)
- `w`: Write-only, create, truncate (O_WRONLY|O_CREATE|O_TRUNC)
- `a`: Write-only, create, append (O_WRONLY|O_CREATE|O_APPEND)
- `r+`: Read-write (O_RDWR)
- `w+`: Read-write, create, truncate (O_RDWR|O_CREATE|O_TRUNC)
- `a+`: Read-write, create, append (O_RDWR|O_CREATE|O_APPEND)

**Top-Level Execution Control**:
- `true`: Top-level llmcmd execution. Direct access to real filesystem allowed
- `false`: Child process execution. Access only to VFS restricted environment

#### Response

**Success**:
```
OK fileno\n
```
- `fileno`: Assigned file number

**Error**:
```
ERROR message\n
```

**Error Patterns**:
- `ERROR OPEN requires filename, mode, and is_top_level` - Missing parameters
- `ERROR invalid mode: invalid` - Invalid mode
- `ERROR invalid is_top_level: maybe` - Invalid top-level flag
- `ERROR VFS not available` - VFS unavailable
- `ERROR failed to open file 'path': reason` - File open error

#### Examples

```
# Success case (top-level)
Client → Server: "OPEN test.txt w true\n"
Server → Client: "OK 12345\n"

# Success case (child process)
Client → Server: "OPEN test.txt w false\n"
Server → Client: "OK 12346\n"

# Error case
Client → Server: "OPEN test.txt invalid true\n"
Server → Client: "ERROR invalid mode: invalid\n"
```

### 2. READ Command

Reads specified bytes from a file number.

#### Request
```
READ fileno size\n
```

**Parameters**:
- `fileno`: File number
- `size`: Number of bytes to read

#### Response

**Success**:
```
OK actual_size\n
[binary_data]
```
- `actual_size`: Actual number of bytes read
- `binary_data`: Read data (actual_size bytes)

**EOF**:
```
OK 0\n
```

**Error**:
```
ERROR message\n
```

**Error Patterns**:
- `ERROR READ requires fileno and size` - Missing parameters
- `ERROR invalid fileno: 99999` - Invalid file number
- `ERROR invalid size: abc` - Invalid size
- `ERROR READ not yet implemented` - Not implemented (Phase 1)

### 3. WRITE Command

Writes specified data to a file number.

#### Request
```
WRITE fileno size\n
[binary_data]
```

**Parameters**:
- `fileno`: File number
- `size`: Number of bytes to write
- `binary_data`: Data to write (size bytes)

#### Response

**Success**:
```
OK written_size\n
```
- `written_size`: Actual number of bytes written

**Error**:
```
ERROR message\n
```

**Error Patterns**:
- `ERROR WRITE requires fileno and size` - Missing parameters
- `ERROR invalid fileno: 99999` - Invalid file number
- `ERROR invalid size: abc` - Invalid size
- `ERROR failed to read data: reason` - Data read error
- `ERROR WRITE not yet implemented` - Not implemented (Phase 1)

### 4. CLOSE Command

Closes a file number.

#### Request
```
CLOSE fileno\n
```

**Parameters**:
- `fileno`: File number

#### Response

**Success**:
```
OK\n
```

**Error**:
```
ERROR message\n
```

**Error Patterns**:
- `ERROR CLOSE requires fileno` - Missing parameters
- `ERROR invalid fileno: abc` - Invalid file number
- `ERROR CLOSE not yet implemented` - Not implemented (Phase 1)

### 5. LLM_CHAT Command (New)

Executes OpenAI ChatCompletion API and retrieves LLM responses. Provides equivalent functionality to llmcmd command-line options.

#### Request
```
LLM_CHAT is_top_level input_files_count output_files_count prompt_length preset_length\n
[input_files_text]
[output_files_text]
[prompt_text]
[preset_text]
```

**Parameters**:
- `is_top_level`: Top-level execution flag ("true" or "false")
- `input_files_count`: Byte count of input files text
- `output_files_count`: Byte count of output files text
- `prompt_length`: Byte count of prompt text
- `preset_length`: Byte count of preset key
- `input_files_text`: Input file paths separated by newlines
- `output_files_text`: Output file paths separated by newlines
- `prompt_text`: User instruction text (equivalent to -p/--prompt)
- `preset_text`: System prompt preset key (equivalent to -r/--preset)

**Model Selection Logic**:
- `is_top_level=true`: Uses default model from config file (user-specified model)
- `is_top_level=false`: Fixed to "gpt-4o-mini" (child process restriction)

**Configuration Control**:
- **temperature**: Uses config file or hardcoded values
- **max_tokens**: Uses config file or hardcoded values
- **Other API parameters**: Retrieved from configuration file

**File Format**:
```
# input_files_text (newline-separated)
/path/to/input1.txt
/path/to/input2.txt
-

# output_files_text (newline-separated)
/path/to/output1.txt
/path/to/output2.txt

# prompt_text (free text)
Analyze the input data and generate a summary report.

# preset_text (system prompt preset key)
data_proc
```

#### Response

**Success**:
```
OK response_size quota_status\n
[response_json]
```
- `response_size`: Byte count of response JSON
- `quota_status`: Quota usage status (e.g., "1250.5/5000 weighted tokens")
- `response_json`: ChatCompletion response JSON

**Error**:
```
ERROR message\n
```

**Error Patterns**:
- `ERROR LLM_CHAT requires is_top_level, input_files_count, output_files_count, prompt_length, and preset_length` - Missing parameters
- `ERROR invalid is_top_level: maybe` - Invalid top-level flag
- `ERROR quota exceeded: cannot make LLM call` - Quota exceeded
- `ERROR OpenAI API call failed: reason` - API call error
- `ERROR LLM not available` - LLM functionality unavailable
- `ERROR failed to read input files data` - Input files data read error
- `ERROR failed to read output files data` - Output files data read error
- `ERROR failed to read prompt data` - Prompt data read error
- `ERROR failed to read preset data` - Preset data read error

#### Examples

```
# Success case (top-level execution - prompt specified)
Client → Server: "LLM_CHAT true 25 12 45 9\n/tmp/input.txt\n-\noutput.txt\nAnalyze the input data and create a summary.\ndata_proc"
Server → Client: "OK 156 1250.5/5000 weighted tokens\n{\"choices\":[{\"message\":{\"content\":\"Analysis complete...\"}}],\"usage\":{\"prompt_tokens\":15,\"completion_tokens\":8}}"

# Success case (child process execution - preset specified)
Client → Server: "LLM_CHAT false 0 0 0 7\n\n\n\ngeneral"
Server → Client: "OK 142 1350.0/5000 weighted tokens\n{\"choices\":[{\"message\":{\"content\":\"Simple task completed\"}}],\"usage\":{\"prompt_tokens\":8,\"completion_tokens\":6}}"

# Error case
Client → Server: "LLM_CHAT invalid 0 0 0 0\n"
Server → Client: "ERROR invalid is_top_level: invalid\n"
```

### 6. LLM_QUOTA Command (New)

Checks current quota usage status.

#### Request
```
LLM_QUOTA\n
```

#### Response

**Success**:
```
OK quota_info\n
```
- `quota_info`: Detailed quota information (e.g., "1250.5/5000 weighted tokens (25.0% used, 3749.5 remaining)")

**Error**:
```
ERROR message\n
```

**Error Patterns**:
- `ERROR LLM quota not available` - Quota functionality unavailable

#### Examples

```
Client → Server: "LLM_QUOTA\n"
Server → Client: "OK 1250.5/5000 weighted tokens (25.0% used, 3749.5 remaining)\n"
```

### 7. LLM_CONFIG Command (New)

Retrieves LLM configuration information.

#### Request
```
LLM_CONFIG\n
```

#### Response

**Success**:
```
OK config_size\n
[config_json]
```
- `config_size`: Byte count of configuration JSON
- `config_json`: LLM configuration information JSON

**Configuration Information Format**:
```json
{
  "default_model": "gpt-4o-mini",
  "api_key_configured": true,
  "base_url": "https://api.openai.com/v1",
  "max_calls": 50,
  "quota_max_tokens": 5000,
  "quota_weights": {
    "input": 1.0,
    "cached": 0.25,
    "output": 4.0
  }
}
```

**Error**:
```
ERROR message\n
```

**Error Patterns**:
- `ERROR LLM config not available` - LLM configuration unavailable

#### Examples

```
Client → Server: "LLM_CONFIG\n"
Server → Client: "OK 234\n{\"default_model\":\"gpt-4o-mini\",\"api_key_configured\":true...}"
```

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
- 🆕 LLM_CONFIG command implementation
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

// Check LLM configuration
config, _ := client.LLMConfig()
fmt.Printf("Default model: %s\n", config.DefaultModel)

// Check quota
quota, _ := client.LLMQuota()
fmt.Printf("Quota: %s\n", quota)

// LLM execution (top-level - prompt specified)
inputFiles := "/tmp/prompt.txt\n/tmp/data.csv"
outputFiles := "/tmp/result.txt"
prompt := "Analyze the input data and generate a summary report."
preset := "" // No preset used
response, _ := client.LLMChat(true, inputFiles, outputFiles, prompt, preset)
fmt.Printf("Response: %s\n", response.Choices[0].Message.Content)

// LLM execution (top-level - preset specified)
response2, _ := client.LLMChat(true, "", "", "", "data_proc")
fmt.Printf("Response: %s\n", response2.Choices[0].Message.Content)

// LLM execution (child process - restricted mode)
response3, _ := client.LLMChat(false, "", "", "Simple calculation: 2+2", "")
fmt.Printf("Response: %s\n", response3.Choices[0].Message.Content)

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
