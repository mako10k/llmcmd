#!/bin/bash

# FSProxy specific test utilities
# Provides common functions for FSProxy protocol testing

# Setup test environment with proper security configuration
setup_fsproxy_test_env() {
    local test_name="$1"
    local execution_mode="$2"  # llmcmd|llmsh-virtual|llmsh-real
    
    export TEST_ENV_DIR="/tmp/fsproxy_test_$$_$test_name"
    mkdir -p "$TEST_ENV_DIR"/{input,output,temp}
    
    # Configure based on execution mode
    case "$execution_mode" in
        "llmcmd")
            export FSPROXY_MODE="llmcmd"
            export FSPROXY_VIRTUAL="false"
            ;;
        "llmsh-virtual")
            export FSPROXY_MODE="llmsh-virtual"
            export FSPROXY_VIRTUAL="true"
            ;;
        "llmsh-real")
            export FSPROXY_MODE="llmsh-real"
            export FSPROXY_VIRTUAL="false"
            ;;
        *)
            echo "ERROR: Unknown execution mode: $execution_mode"
            exit 1
            ;;
    esac
    
    echo "Test environment setup: $TEST_ENV_DIR (mode: $execution_mode)"
}

# Verify security policy enforcement
verify_access_control() {
    local file="$1"
    local mode="$2"
    local should_succeed="$3"
    
    # Test file access and verify result matches expectation
    if [[ "$should_succeed" == "true" ]]; then
        assert_file_accessible "$file" "$mode"
    else
        assert_access_denied "$file" "$mode"
    fi
}

# Assert file is accessible with given mode
assert_file_accessible() {
    local file="$1"
    local mode="$2"
    
    case "$mode" in
        "read")
            if ! cat "$file" >/dev/null 2>&1; then
                echo "ERROR: File should be readable: $file"
                exit 1
            fi
            ;;
        "write")
            if ! echo "test" > "$file" 2>/dev/null; then
                echo "ERROR: File should be writable: $file"
                exit 1
            fi
            ;;
        *)
            echo "ERROR: Unknown access mode: $mode"
            exit 1
            ;;
    esac
}

# Assert access is denied for file with given mode
assert_access_denied() {
    local file="$1"
    local mode="$2"
    
    case "$mode" in
        "read")
            if cat "$file" >/dev/null 2>&1; then
                echo "ERROR: File read should be denied: $file"
                exit 1
            fi
            ;;
        "write")
            if echo "test" > "$file" 2>/dev/null; then
                echo "ERROR: File write should be denied: $file"
                exit 1
            fi
            ;;
        *)
            echo "ERROR: Unknown access mode: $mode"
            exit 1
            ;;
    esac
}

# Monitor resource usage during test
monitor_resources() {
    local test_pid="$1"
    local monitor_file="$2"
    
    echo "# Resource monitoring for PID $test_pid" > "$monitor_file"
    echo "# Format: timestamp pid ppid rss vsz pcpu comm" >> "$monitor_file"
    
    while kill -0 "$test_pid" 2>/dev/null; do
        echo "$(date '+%Y-%m-%d %H:%M:%S'): $(ps -o pid,ppid,rss,vsz,pcpu,comm -p "$test_pid" --no-headers 2>/dev/null || echo 'Process not found')" >> "$monitor_file"
        sleep 1
    done
}

# Cleanup test environment
cleanup_test_env() {
    if [[ -n "${TEST_ENV_DIR:-}" && -d "$TEST_ENV_DIR" ]]; then
        echo "Cleaning up test environment: $TEST_ENV_DIR"
        rm -rf "$TEST_ENV_DIR"
    fi
}

# Check if binary exists and is executable
check_binary() {
    local binary_path="$1"
    
    if [[ ! -f "$binary_path" ]]; then
        echo "ERROR: Binary not found: $binary_path"
        exit 1
    fi
    
    if [[ ! -x "$binary_path" ]]; then
        echo "ERROR: Binary not executable: $binary_path"
        exit 1
    fi
}

# Create test files with specific content
create_test_files() {
    local base_dir="$1"
    local file_type="$2"  # text|binary|large|csv
    
    case "$file_type" in
        "text")
            cat > "$base_dir/test.txt" << 'EOF'
This is a sample text file for testing.
It contains multiple lines.
Each line has different content for verification.
EOF
            ;;
        "binary")
            # Create a small binary file
            dd if=/dev/urandom of="$base_dir/test.bin" bs=1024 count=1 2>/dev/null
            ;;
        "large")
            # Create a larger file for performance testing
            for i in {1..1000}; do
                echo "Line $i: This is line number $i in the large test file."
            done > "$base_dir/large.txt"
            ;;
        "csv")
            cat > "$base_dir/test.csv" << 'EOF'
Name,Age,City,Score
Alice,25,Tokyo,85.5
Bob,30,Osaka,92.0
Charlie,35,Kyoto,78.5
Diana,28,Yokohama,89.0
EOF
            ;;
        *)
            echo "ERROR: Unknown file type: $file_type"
            exit 1
            ;;
    esac
}

# Validate test result format
validate_result_format() {
    local result="$1"
    local expected_type="$2"  # json|text|csv
    
    case "$expected_type" in
        "json")
            if ! echo "$result" | jq . >/dev/null 2>&1; then
                echo "ERROR: Result is not valid JSON"
                return 1
            fi
            ;;
        "text")
            if [[ -z "$result" ]]; then
                echo "ERROR: Result is empty"
                return 1
            fi
            ;;
        "csv")
            if ! echo "$result" | grep -q "," ; then
                echo "ERROR: Result does not appear to be CSV format"
                return 1
            fi
            ;;
        *)
            echo "ERROR: Unknown result type: $expected_type"
            return 1
            ;;
    esac
    
    return 0
}

# Common test patterns
run_test_with_timeout() {
    local timeout_seconds="$1"
    local command="$2"
    
    if timeout "$timeout_seconds" bash -c "$command"; then
        return 0
    else
        local exit_code=$?
        if [[ $exit_code -eq 124 ]]; then
            echo "ERROR: Command timed out after $timeout_seconds seconds"
        else
            echo "ERROR: Command failed with exit code: $exit_code"
        fi
        return $exit_code
    fi
}
