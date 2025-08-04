# FSProxy Protocol Test-First Implementation Plan

## 🎯 Overview

This document outlines a comprehensive test-first development approach for implementing the fsproxy protocol in llmcmd, following existing test architecture patterns while ensuring complete coverage of all protocol requirements.

## 📋 Current Test Architecture Assessment

### Existing Framework Strengths
- **Shell Integration Testing**: Comprehensive framework in `tests/` directory
- **Go Unit Testing**: Scattered but well-structured patterns across `internal/` packages  
- **4-Factor MVP Testing**: Proven approach in security module (Core, Error, Data, Performance)
- **Table-Driven Tests**: Standard Go testing patterns throughout codebase
- **Mock Infrastructure**: OpenAI API mocking for deterministic testing

### Test Pattern Analysis
```go
// Existing Pattern: Table-driven tests with comprehensive coverage
func TestParseArgs(t *testing.T) {
    tests := []struct {
        name    string
        args    []string
        want    *Config
        wantErr error
    }{
        // Test cases...
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation...
        })
    }
}

// Existing Pattern: 4-Factor MVP testing approach
func TestCriticalFactors(t *testing.T) {
    t.Run("Factor1_CoreFunctionality", func(t *testing.T) { /* ... */ })
    t.Run("Factor2_ErrorHandling", func(t *testing.T) { /* ... */ })
    t.Run("Factor3_DataIntegrity", func(t *testing.T) { /* ... */ })
    t.Run("Factor4_PerformanceAndSecurity", func(t *testing.T) { /* ... */ })
}
```

## 🧪 Test-First Implementation Strategy

### Phase 1: Foundation Tests (Week 1)

#### 1.1 Protocol Message Parser Tests
**File**: `internal/app/fsproxy_test.go`

```go
func TestFSProxyMessageParser(t *testing.T) {
    tests := []struct {
        name        string
        input       string
        expected    FSProxyMessage
        expectError bool
    }{
        {
            name:  "valid_open_command",
            input: "OPEN /path/file.txt r",
            expected: FSProxyMessage{
                Command:   "OPEN",
                Path:      "/path/file.txt", 
                Mode:      "r",
                RequestID: "",
            },
            expectError: false,
        },
        {
            name:        "invalid_command_format",
            input:       "INVALID",
            expectError: true,
        },
        // More test cases for all 7 commands...
    }
}
```

#### 1.2 VFS Integration Tests  
**File**: `internal/app/vfs_fsproxy_test.go`

```go
func TestVFSFSProxyIntegration_CriticalFactors(t *testing.T) {
    t.Run("Factor1_CoreFunctionality", func(t *testing.T) {
        // Test basic file operations through fsproxy
    })
    t.Run("Factor2_ErrorHandling", func(t *testing.T) {
        // Test error propagation from VFS to fsproxy
    })
    t.Run("Factor3_DataIntegrity", func(t *testing.T) {
        // Test file descriptor mapping consistency
    })
    t.Run("Factor4_PerformanceAndSecurity", func(t *testing.T) {
        // Test resource cleanup and security boundaries
    })
}
```

### Phase 2: Command Implementation Tests (Week 2)

#### 2.1 Individual Command Tests
**File**: `internal/app/fsproxy_commands_test.go`

```go
func TestFSProxyCommands(t *testing.T) {
    commands := []struct {
        name string
        test func(t *testing.T, proxy *FSProxyManager)
    }{
        {"OPEN", testOpenCommand},
        {"WRITE", testWriteCommand}, 
        {"READ", testReadCommand},
        {"SEEK", testSeekCommand},
        {"CLOSE", testCloseCommand},
        {"EXECUTE", testExecuteCommand},
        {"EXIT", testExitCommand},
    }
    
    for _, cmd := range commands {
        t.Run(cmd.name, func(t *testing.T) {
            proxy := setupTestFSProxy(t)
            defer proxy.Cleanup()
            cmd.test(t, proxy)
        })
    }
}
```

#### 2.2 LLM Integration Tests
**File**: `internal/app/fsproxy_llm_test.go`

```go
func TestFSProxyLLMIntegration(t *testing.T) {
    // Mock OpenAI client for deterministic testing
    mockClient := &MockOpenAIClient{
        responses: []MockResponse{
            {
                FunctionCall: &FunctionCall{
                    Name: "WRITE",
                    Arguments: `{"fd": 3, "data": "test content"}`,
                },
            },
        },
    }
    
    proxy := NewFSProxyManager(mockClient)
    
    // Test LLM command execution through fsproxy
    result, err := proxy.ExecuteLLMCommand("process file")
    assert.NoError(t, err)
    assert.Contains(t, result, "test content")
}
```

### Phase 3: Error Handling & Edge Cases (Week 3)

#### 3.1 Error Scenario Tests
**File**: `internal/app/fsproxy_errors_test.go`

```go
func TestFSProxyErrorHandling(t *testing.T) {
    errorScenarios := []struct {
        name          string
        setup         func(*FSProxyManager)
        command       string
        expectedError string
    }{
        {
            name:          "invalid_file_descriptor",
            command:       "READ 999",
            expectedError: "invalid file descriptor",
        },
        {
            name:          "write_to_readonly_fd",
            setup:         func(p *FSProxyManager) { /* setup readonly fd */ },
            command:       "WRITE 3 data",
            expectedError: "permission denied",
        },
        // More error scenarios...
    }
}
```

#### 3.2 Resource Management Tests
**File**: `internal/app/fsproxy_resources_test.go`

```go
func TestFSProxyResourceManagement(t *testing.T) {
    t.Run("file_descriptor_limits", func(t *testing.T) {
        // Test FD limit enforcement
    })
    t.Run("memory_usage_tracking", func(t *testing.T) {
        // Test memory limit enforcement  
    })
    t.Run("cleanup_on_exit", func(t *testing.T) {
        // Test proper resource cleanup
    })
}
```

### Phase 4: Integration & Performance Tests (Week 4)

#### 4.1 End-to-End Integration Tests
**File**: `tests/integration/fsproxy/e2e_test.sh`

```bash
#!/bin/bash
# Following existing test framework patterns

source "$(dirname "$0")/../../framework/helpers.sh"
source "$(dirname "$0")/../../framework/assertions.sh"

test_fsproxy_basic_workflow() {
    local config_file=$(create_test_config '{
        "fsproxy_mode": true,
        "model": "gpt-4o-mini"
    }')
    
    local result=$(run_llmcmd -c "$config_file" "process test file")
    
    assert_success "$result"
    assert_output_contains "$result" "file processed"
}

test_fsproxy_error_recovery() {
    # Test error recovery scenarios
}
```

#### 4.2 Performance & Benchmark Tests
**File**: `internal/app/fsproxy_benchmark_test.go`

```go
func BenchmarkFSProxyMessageParsing(b *testing.B) {
    message := "WRITE 3 large_data_content_here"
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := ParseFSProxyMessage(message)
        if err != nil {
            b.Fatal(err)
        }
    }
}

func BenchmarkFSProxyVFSOperations(b *testing.B) {
    // Benchmark VFS operations through fsproxy
}
```

## 🛠️ Test Infrastructure Setup

### Mock Components Required

#### 1. Mock OpenAI Client
```go
type MockOpenAIClient struct {
    responses []MockResponse
    callCount int
}

func (m *MockOpenAIClient) CreateChatCompletion(req ChatCompletionRequest) (*ChatCompletionResponse, error) {
    if m.callCount >= len(m.responses) {
        return nil, fmt.Errorf("unexpected API call")
    }
    
    response := m.responses[m.callCount]
    m.callCount++
    return response.ToResponse(), nil
}
```

#### 2. Test VFS Implementation
```go
type TestVFS struct {
    files map[string][]byte
    fds   map[int]*TestFileDescriptor
}

func (v *TestVFS) OpenFile(path string, mode int) (int, error) {
    // Simplified VFS for testing
}
```

### Test Fixtures

#### 1. Protocol Message Fixtures
```
fixtures/fsproxy/
├── valid_messages.txt       # Valid protocol messages
├── invalid_messages.txt     # Invalid protocol messages  
├── llm_responses.json       # Mock LLM responses
└── expected_outputs.txt     # Expected command outputs
```

#### 2. Configuration Fixtures
```go
// Test configurations for different scenarios
var TestConfigs = map[string]*Config{
    "fsproxy_basic": {
        FSProxyMode: true,
        Model:       "gpt-4o-mini",
        MaxAPICalls: 10,
    },
    "fsproxy_restricted": {
        FSProxyMode: true,
        FileSystemAccess: "readonly",
    },
}
```

## 📊 Test Coverage Requirements

### Coverage Targets
- **Unit Tests**: 90%+ coverage for all fsproxy components
- **Integration Tests**: 100% command coverage
- **Error Scenarios**: All error paths tested
- **Performance Tests**: Baseline benchmarks established

### Critical Test Areas
1. **Protocol Parsing**: All 7 commands with valid/invalid inputs
2. **VFS Integration**: File operations through fsproxy interface
3. **LLM Communication**: OpenAI function calling integration
4. **Error Handling**: All error conditions and recovery
5. **Resource Management**: Memory limits, FD limits, cleanup
6. **Security**: Access control and isolation

## 🔄 Test Execution Strategy

### Development Workflow
1. **Write Tests First**: Before implementing any feature
2. **Red-Green-Refactor**: Standard TDD cycle
3. **Continuous Testing**: Run tests on every change
4. **Integration Validation**: E2E tests for major milestones

### Validation Gates
- **Unit Tests**: Must pass before code integration
- **Integration Tests**: Must pass before feature completion
- **Performance Tests**: Must meet baseline requirements
- **Security Tests**: Must pass security validation

## 📝 Implementation Checklist

### Week 1: Foundation
- [ ] Create `internal/app/fsproxy_test.go`
- [ ] Implement protocol message parser tests
- [ ] Create VFS integration test framework
- [ ] Setup mock OpenAI client infrastructure

### Week 2: Commands
- [ ] Implement tests for all 7 fsproxy commands
- [ ] Create LLM integration test scenarios
- [ ] Validate OpenAI function calling integration
- [ ] Test file descriptor management

### Week 3: Error Handling
- [ ] Create comprehensive error scenario tests
- [ ] Implement resource management tests
- [ ] Validate security boundary tests
- [ ] Test cleanup and recovery mechanisms

### Week 4: Integration
- [ ] Create end-to-end integration tests
- [ ] Implement performance benchmarks
- [ ] Validate complete protocol compliance
- [ ] Document test results and coverage

## 🎯 Success Criteria

### Technical Validation
- All tests pass with 90%+ coverage
- Performance meets or exceeds baseline
- No security vulnerabilities identified
- Complete protocol compliance validated

### Quality Assurance
- Test suite is maintainable and extensible
- Clear error messages and debugging information
- Comprehensive documentation for all test scenarios
- Integration with existing CI/CD pipeline

This test-first approach ensures robust implementation of the fsproxy protocol while maintaining the high quality standards established in the existing llmcmd codebase.
