# Testing Framework Specification

## Overview

This document outlines the comprehensive testing framework for the llmcmd and llmsh tools, implementing Phase 1-2 of the testing strategy.

## Architecture

### Framework Structure
```
tests/
├── framework/                 # Core testing framework
│   ├── test_runner.sh        # Main test execution engine
│   ├── helpers.sh            # Utility functions for test execution
│   └── assertions.sh         # Test assertion library
├── integration/              # Integration tests
│   ├── llmsh/               # llmsh-specific tests
│   │   ├── basic_commands/   # Basic command functionality
│   │   ├── pipelines/        # Pipeline processing tests
│   │   ├── virtual_mode/     # Virtual mode restrictions
│   │   └── error_handling/   # Error scenario handling
│   ├── llmcmd/              # llmcmd-specific tests
│   │   ├── tool_execution/   # Tool execution engine tests
│   │   ├── builtin_commands/ # Built-in command tests
│   │   └── error_handling/   # Configuration and API error tests
│   └── scenarios/           # Real-world usage scenarios
├── fixtures/                # Test data and configurations
│   ├── input/               # Sample input files
│   ├── expected/            # Expected output files
│   └── configs/             # Test configuration files
├── unit/                    # Unit tests (future implementation)
└── reports/                 # Test execution reports
```

## Test Categories

### llmsh Integration Tests

#### 1. Basic Commands (`basic_commands/`)
- **help_version.sh**: Help and version information
- **command_execution.sh**: Basic shell command execution
- **io_redirection.sh**: Input/output redirection testing

#### 2. Pipeline Processing (`pipelines/`)
- **simple_pipes.sh**: Basic pipe operations
- **complex_pipes.sh**: Advanced pipeline scenarios
- **pipe_error_handling.sh**: Error propagation in pipes

#### 3. Virtual Mode (`virtual_mode/`)
- **virtual_restrictions.sh**: File access and command restrictions
- **virtual_io.sh**: Input/output handling in virtual mode

#### 4. Error Handling (`error_handling/`)
- **error_scenarios.sh**: Various error conditions and responses

### llmcmd Integration Tests

#### 1. Tool Execution (`tool_execution/`)
- **basic_tools.sh**: Read, write, spawn, and exit tools
- **advanced_tools.sh**: Complex tool chaining and workflows

#### 2. Built-in Commands (`builtin_commands/`)
- **text_processing.sh**: Cat, grep, sort, wc, and other text tools

#### 3. Error Handling (`error_handling/`)
- **config_errors.sh**: Configuration and API error scenarios

### Scenario Tests (`scenarios/`)
- **real_world_usage.sh**: Realistic usage patterns
- **performance_edge_cases.sh**: Performance and edge case testing

## Framework Components

### Test Runner (`test_runner.sh`)
Main execution engine supporting:
- Dual-tool testing (llmcmd and llmsh)
- Selective test execution by tool or pattern
- Colored output and comprehensive reporting
- List mode for test discovery

#### Usage Examples:
```bash
./run_tests.sh                              # Run all tests
./run_tests.sh --tool llmsh                 # Run all llmsh tests
./run_tests.sh --tool llmcmd --test basic   # Run llmcmd tests matching 'basic'
./run_tests.sh --list                       # List all available tests
./run_tests.sh --verbose                    # Run with verbose output
```

### Helper Functions (`helpers.sh`)
Core utilities including:
- `run_llmcmd()` - Execute llmcmd with standardized output capture
- `run_llmsh()` - Execute llmsh with option parsing
- `create_temp_file()` - Temporary file management
- `cleanup()` - Resource cleanup

### Assertion Library (`assertions.sh`)
Comprehensive assertion functions:
- `assert_success()` - Verify command success
- `assert_failure()` - Verify command failure
- `assert_output_contains()` - Check output content
- `assert_file_exists()` - File existence verification
- `assert_string_contains()` - String matching

## Test Execution Flow

### 1. Environment Setup
- Binary path validation
- Test configuration loading
- Temporary directory creation

### 2. Test Discovery
- Scan test directories for executable test files
- Apply filters for tool and pattern matching
- Build execution queue

### 3. Test Execution
- Source framework components
- Execute individual test functions
- Capture results and output

### 4. Reporting
- Aggregate test results
- Generate summary statistics
- Provide detailed failure information

## Configuration

### Environment Variables
- `LLMCMD_BINARY`: Path to llmcmd executable
- `LLMSH_BINARY`: Path to llmsh executable
- `TEST_VERBOSE`: Enable verbose output
- `TEST_TIMEOUT`: Test execution timeout

### Test Data
- **Sample Files**: Various input files for testing file operations
- **Configuration Files**: Valid and invalid configuration examples
- **Expected Outputs**: Reference outputs for validation

## Implementation Status

### ✅ Completed (Phase 1-2)
- Test framework infrastructure
- Helper and assertion libraries
- Basic command tests for llmsh
- Pipeline processing tests
- Virtual mode restriction tests
- Tool execution tests for llmcmd
- Built-in command tests
- Error handling scenarios
- Real-world usage scenarios
- Performance and edge case tests

### 🔄 Future Enhancements (Phase 3-4)
- Unit tests for Go components
- Performance benchmarking
- Load testing scenarios
- Continuous integration integration
- Test result persistence
- Coverage reporting

## Quality Assurance

### Test Design Principles
1. **Isolation**: Each test runs in a clean environment
2. **Determinism**: Tests produce consistent results
3. **Coverage**: Comprehensive scenario coverage
4. **Maintainability**: Clear, readable test code
5. **Efficiency**: Fast execution for rapid feedback

### Validation Methods
- Exit code verification
- Output content matching
- File system state checking
- Error message validation
- Performance threshold monitoring

## Maintenance Guidelines

### Adding New Tests
1. Create test file in appropriate category directory
2. Source framework components
3. Implement test functions with descriptive names
4. Use standardized assertion methods
5. Include setup and teardown procedures

### Test Naming Conventions
- Files: `category_description.sh`
- Functions: `test_specific_functionality()`
- Variables: `UPPERCASE_CONSTANTS`, `lowercase_locals`

### Documentation Requirements
- Function-level comments explaining test purpose
- Complex logic documentation
- Expected behavior descriptions
- Error condition coverage notes

## Integration with CI/CD

The testing framework is designed for integration with continuous integration systems:

```bash
# CI execution example
./tests/run_tests.sh --tool all > test_results.log 2>&1
if [ $? -eq 0 ]; then
    echo "All tests passed"
else
    echo "Tests failed - see test_results.log"
    exit 1
fi
```

## Conclusion

This testing framework provides comprehensive coverage for both llmcmd and llmsh tools, ensuring reliability, maintainability, and quality assurance. The modular design supports incremental enhancement and easy maintenance while providing robust validation of all major functionality.
