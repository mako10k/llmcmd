#!/bin/bash

# Test Helper Functions for llmcmd/llmsh

# Run llmcmd with arguments and capture output
run_llmcmd() {
    local output_file="$TEMP_DIR/llmcmd_output_$$"
    local error_file="$TEMP_DIR/llmcmd_error_$$"
    local exit_code
    
    "$LLMCMD_BINARY" "$@" > "$output_file" 2> "$error_file"
    exit_code=$?
    
    # Store results in global variables
    LAST_OUTPUT=$(cat "$output_file" 2>/dev/null || echo "")
    LAST_ERROR=$(cat "$error_file" 2>/dev/null || echo "")
    LAST_EXIT_CODE=$exit_code
    
    rm -f "$output_file" "$error_file"
    return $exit_code
}

# Run llmsh with arguments and capture output
run_llmsh() {
    local output_file="$TEMP_DIR/llmsh_output_$$"
    local error_file="$TEMP_DIR/llmsh_error_$$"
    local exit_code
    
    "$LLMSH_BINARY" "$@" > "$output_file" 2> "$error_file"
    exit_code=$?
    
    # Store results in global variables
    LAST_OUTPUT=$(cat "$output_file" 2>/dev/null || echo "")
    LAST_ERROR=$(cat "$error_file" 2>/dev/null || echo "")
    LAST_EXIT_CODE=$exit_code
    
    rm -f "$output_file" "$error_file"
    return $exit_code
}

# Create a temporary file
create_temp_file() {
    local filename
    if [[ $# -eq 0 ]]; then
        # Generate unique filename if not provided
        filename="temp_$(date +%s)_$$_$RANDOM.txt"
    else
        filename="$1"
    fi
    
    local filepath="$TEMP_DIR/$filename"
    touch "$filepath"
    echo "$filepath"
}

# Write content to a temporary file
write_temp_file() {
    local content="$1"
    local filename="$2"
    local filepath
    
    if [[ -n "$filename" ]]; then
        filepath="$TEMP_DIR/$filename"
    else
        filepath=$(create_temp_file)
    fi
    
    echo -e "$content" > "$filepath"
    echo "$filepath"
}

# Create a test config file
create_test_config() {
    local config_content="$1"
    local config_file="$TEMP_DIR/test_config.json"
    
    echo "$config_content" > "$config_file"
    echo "$config_file"
}

# Clean up test files
cleanup_test_files() {
    rm -f "$TEMP_DIR"/test_* "$TEMP_DIR"/*_output_* "$TEMP_DIR"/*_error_*
}

# Setup test environment for individual tests
setup_test() {
    cleanup_test_files
    mkdir -p "$TEMP_DIR"
}

# Teardown test environment for individual tests
teardown_test() {
    cleanup_test_files
}

# Get the absolute path of a test fixture
get_fixture() {
    local fixture_name="$1"
    echo "$FIXTURES_DIR/$fixture_name"
}

# Check if a file exists and has expected content
file_contains() {
    local filepath="$1"
    local expected_content="$2"
    
    if [[ ! -f "$filepath" ]]; then
        return 1
    fi
    
    grep -q "$expected_content" "$filepath"
}

# Wait for a process with timeout
wait_with_timeout() {
    local pid="$1"
    local timeout="${2:-10}"
    local count=0
    
    while kill -0 "$pid" 2>/dev/null; do
        if [[ $count -ge $timeout ]]; then
            kill -TERM "$pid" 2>/dev/null
            return 1
        fi
        sleep 1
        ((count++))
    done
    
    return 0
}

# Print test debug info
debug_last_run() {
    echo "Exit Code: $LAST_EXIT_CODE"
    echo "Output: $LAST_OUTPUT"
    echo "Error: $LAST_ERROR"
}

# Get test data directory for specific tool
get_test_data_dir() {
    local tool="$1"
    echo "$FIXTURES_DIR/$tool"
}
