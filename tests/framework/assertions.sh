#!/bin/bash

# Test Assertion Functions for llmcmd/llmsh

# Assert that the last command succeeded (exit code 0)
assert_success() {
    local message="${1:-Expected command to succeed}"
    
    if [[ $LAST_EXIT_CODE -eq 0 ]]; then
        return 0
    else
        echo -e "${RED}ASSERTION FAILED: $message${NC}"
        echo -e "${RED}Expected exit code: 0, got: $LAST_EXIT_CODE${NC}"
        debug_last_run
        return 1
    fi
}

# Assert that the last command failed (exit code non-zero)
assert_failure() {
    local message="${1:-Expected command to fail}"
    
    if [[ $LAST_EXIT_CODE -ne 0 ]]; then
        return 0
    else
        echo -e "${RED}ASSERTION FAILED: $message${NC}"
        echo -e "${RED}Expected non-zero exit code, got: $LAST_EXIT_CODE${NC}"
        debug_last_run
        return 1
    fi
}

# Assert that the last command had specific exit code
assert_exit_code() {
    local expected_code="$1"
    local message="${2:-Expected exit code $expected_code}"
    
    if [[ $LAST_EXIT_CODE -eq $expected_code ]]; then
        return 0
    else
        echo -e "${RED}ASSERTION FAILED: $message${NC}"
        echo -e "${RED}Expected exit code: $expected_code, got: $LAST_EXIT_CODE${NC}"
        debug_last_run
        return 1
    fi
}

# Assert that output contains expected string
assert_output_contains() {
    local expected="$1"
    local message="${2:-Expected output to contain '$expected'}"
    
    if echo "$LAST_OUTPUT" | grep -q "$expected"; then
        return 0
    else
        echo -e "${RED}ASSERTION FAILED: $message${NC}"
        echo -e "${RED}Output does not contain: $expected${NC}"
        echo -e "${RED}Actual output: $LAST_OUTPUT${NC}"
        return 1
    fi
}

# Assert that output equals expected string
assert_output_equals() {
    local expected="$1"
    local message="${2:-Expected output to equal '$expected'}"
    
    if [[ "$LAST_OUTPUT" == "$expected" ]]; then
        return 0
    else
        echo -e "${RED}ASSERTION FAILED: $message${NC}"
        echo -e "${RED}Expected: $expected${NC}"
        echo -e "${RED}Actual: $LAST_OUTPUT${NC}"
        return 1
    fi
}

# Assert that error output contains expected string
assert_error_contains() {
    local expected="$1"
    local message="${2:-Expected error to contain '$expected'}"
    
    if echo "$LAST_ERROR" | grep -q "$expected"; then
        return 0
    else
        echo -e "${RED}ASSERTION FAILED: $message${NC}"
        echo -e "${RED}Error does not contain: $expected${NC}"
        echo -e "${RED}Actual error: $LAST_ERROR${NC}"
        return 1
    fi
}

# Assert that output is empty
assert_output_empty() {
    local message="${1:-Expected output to be empty}"
    
    if [[ -z "$LAST_OUTPUT" ]]; then
        return 0
    else
        echo -e "${RED}ASSERTION FAILED: $message${NC}"
        echo -e "${RED}Expected empty output, got: $LAST_OUTPUT${NC}"
        return 1
    fi
}

# Assert that error output is empty
assert_error_empty() {
    local message="${1:-Expected error to be empty}"
    
    if [[ -z "$LAST_ERROR" ]]; then
        return 0
    else
        echo -e "${RED}ASSERTION FAILED: $message${NC}"
        echo -e "${RED}Expected empty error, got: $LAST_ERROR${NC}"
        return 1
    fi
}

# Assert that a file exists
assert_file_exists() {
    local filepath="$1"
    local message="${2:-Expected file to exist: $filepath}"
    
    if [[ -f "$filepath" ]]; then
        return 0
    else
        echo -e "${RED}ASSERTION FAILED: $message${NC}"
        echo -e "${RED}File does not exist: $filepath${NC}"
        return 1
    fi
}

# Assert that a file does not exist
assert_file_not_exists() {
    local filepath="$1"
    local message="${2:-Expected file to not exist: $filepath}"
    
    if [[ ! -f "$filepath" ]]; then
        return 0
    else
        echo -e "${RED}ASSERTION FAILED: $message${NC}"
        echo -e "${RED}File exists: $filepath${NC}"
        return 1
    fi
}

# Assert that file contains expected content
assert_file_contains() {
    local filepath="$1"
    local expected="$2"
    local message="${3:-Expected file to contain '$expected'}"
    
    if [[ -f "$filepath" ]] && grep -q "$expected" "$filepath"; then
        return 0
    else
        echo -e "${RED}ASSERTION FAILED: $message${NC}"
        echo -e "${RED}File: $filepath${NC}"
        if [[ -f "$filepath" ]]; then
            echo -e "${RED}Content: $(cat "$filepath")${NC}"
        else
            echo -e "${RED}File does not exist${NC}"
        fi
        return 1
    fi
}

# Assert that output matches regex pattern
assert_output_matches() {
    local pattern="$1"
    local message="${2:-Expected output to match pattern '$pattern'}"
    
    if echo "$LAST_OUTPUT" | grep -E -q "$pattern"; then
        return 0
    else
        echo -e "${RED}ASSERTION FAILED: $message${NC}"
        echo -e "${RED}Pattern: $pattern${NC}"
        echo -e "${RED}Actual output: $LAST_OUTPUT${NC}"
        return 1
    fi
}

# Assert that two strings are equal
assert_equals() {
    local actual="$1"
    local expected="$2"
    local message="${3:-Expected '$expected', got '$actual'}"
    
    if [[ "$actual" == "$expected" ]]; then
        return 0
    else
        echo -e "${RED}ASSERTION FAILED: $message${NC}"
        echo -e "${RED}Expected: $expected${NC}"
        echo -e "${RED}Actual: $actual${NC}"
        return 1
    fi
}

# Assert that a number is greater than expected
assert_greater_than() {
    local actual="$1"
    local expected="$2"
    local message="${3:-Expected $actual > $expected}"
    
    if [[ $actual -gt $expected ]]; then
        return 0
    else
        echo -e "${RED}ASSERTION FAILED: $message${NC}"
        echo -e "${RED}Expected: $actual > $expected${NC}"
        return 1
    fi
}
