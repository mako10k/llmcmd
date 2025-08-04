#!/bin/bash

# llmcmd/llmsh Test Framework
# Usage: ./test_runner.sh [tool] [test_category] [specific_test]
#        ./test_runner.sh llmsh basic_commands
#        ./test_runner.sh llmcmd tool_execution
#        ./test_runner.sh all

set -e

# Configuration
# Binary paths - project root relative  
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$(dirname "$SCRIPT_DIR")")"
LLMCMD_BINARY="${LLMCMD_BINARY:-$PROJECT_ROOT/llmcmd}"
LLMSH_BINARY="${LLMSH_BINARY:-$PROJECT_ROOT/llmsh}"
TEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FIXTURES_DIR="$TEST_DIR/fixtures"
TEMP_DIR="$TEST_DIR/temp"
RESULTS_DIR="$TEST_DIR/results"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test statistics
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0
SKIPPED_TESTS=0

# Source helper functions
source "$TEST_DIR/framework/helpers.sh"
source "$TEST_DIR/framework/assertions.sh"

# Setup test environment
setup_test_env() {
    echo -e "${BLUE}Setting up test environment...${NC}"
    
    # Create temporary directories
    mkdir -p "$TEMP_DIR" "$RESULTS_DIR"
    
    # Verify binaries exist and are executable
    if [[ ! -x "$LLMCMD_BINARY" ]]; then
        echo -e "${RED}Error: llmcmd binary not found or not executable: $LLMCMD_BINARY${NC}"
        exit 1
    fi
    
    if [[ ! -x "$LLMSH_BINARY" ]]; then
        echo -e "${RED}Error: llmsh binary not found or not executable: $LLMSH_BINARY${NC}"
        exit 1
    fi
    
    # Get version info
    local llmcmd_version llmsh_version
    llmcmd_version=$("$LLMCMD_BINARY" --version 2>/dev/null || echo "unknown")
    llmsh_version=$("$LLMSH_BINARY" --version 2>/dev/null || echo "unknown")
    echo -e "${BLUE}Testing llmcmd version: $llmcmd_version${NC}"
    echo -e "${BLUE}Testing llmsh version: $llmsh_version${NC}"
}

# Cleanup test environment
cleanup_test_env() {
    echo -e "${BLUE}Cleaning up test environment...${NC}"
    rm -rf "$TEMP_DIR"
}

# Run a specific test file
run_test_file() {
    local test_file="$1"
    local test_name
    test_name=$(basename "$test_file" .sh)
    
    echo -e "${YELLOW}Running test: $test_name${NC}"
    
    # Source the test file in a subshell to isolate variables
    (
        source "$test_file"
        if declare -f "test_$test_name" >/dev/null; then
            "test_$test_name"
        else
            echo -e "${RED}No test function found: test_$test_name${NC}"
            return 1
        fi
    )
    
    local exit_code=$?
    if [[ $exit_code -eq 0 ]]; then
        echo -e "${GREEN}✓ PASS: $test_name${NC}"
        ((PASSED_TESTS++))
    else
        echo -e "${RED}✗ FAIL: $test_name${NC}"
        ((FAILED_TESTS++))
    fi
    
    ((TOTAL_TESTS++))
}

# Run tests in a category
run_test_category() {
    local tool="$1"
    local category="$2"
    local test_filter="$3"
    local category_dir="$TEST_DIR/integration/$tool/$category"
    
    if [[ ! -d "$category_dir" ]]; then
        echo -e "${RED}Test category not found: $tool/$category${NC}"
        return 1
    fi
    
    echo -e "${BLUE}Running test category: $tool/$category${NC}"
    
    # Run all test files in the category
    for test_file in "$category_dir"/*.sh; do
        [[ -f "$test_file" ]] || continue
        local test_name
        test_name=$(basename "$test_file" .sh)
        
        # Apply test filter if specified
        if [[ -z "$test_filter" || "$test_name" == *"$test_filter"* ]]; then
            run_test_file "$test_file"
        fi
    done
}

# Run all tests for a tool
run_tool_tests() {
    local tool_filter="$1"
    local test_filter="$2"
    
    if [[ -n "$tool_filter" && "$tool_filter" != "all" ]]; then
        echo -e "${BLUE}Running tests for tool: $tool_filter${NC}"
        for category_dir in "$TEST_DIR/integration/$tool_filter"/*; do
            [[ -d "$category_dir" ]] || continue
            local category
            category=$(basename "$category_dir")
            
            if [[ -z "$test_filter" || "$category" == *"$test_filter"* ]]; then
                run_test_category "$tool_filter" "$category" "$test_filter"
            else
                # Check if any test file in category matches the filter
                local found_match=false
                for test_file in "$category_dir"/*.sh; do
                    [[ -f "$test_file" ]] || continue
                    local test_name
                    test_name=$(basename "$test_file" .sh)
                    if [[ "$test_name" == *"$test_filter"* ]]; then
                        found_match=true
                        break
                    fi
                done
                if [[ "$found_match" == "true" ]]; then
                    run_test_category "$tool_filter" "$category" "$test_filter"
                fi
            fi
        done
    else
        # Run all tests
        echo -e "${BLUE}Running all tests${NC}"
        for tool_dir in "$TEST_DIR/integration"/*; do
            [[ -d "$tool_dir" ]] || continue
            local tool
            tool=$(basename "$tool_dir")
            
            for category_dir in "$tool_dir"/*; do
                [[ -d "$category_dir" ]] || continue
                local category
                category=$(basename "$category_dir")
                
                if [[ -z "$test_filter" || "$category" == *"$test_filter"* ]]; then
                    run_test_category "$tool" "$category" "$test_filter"
                else
                    # Check if any test file in category matches the filter
                    local found_match=false
                    for test_file in "$TEST_DIR/integration/$tool/$category"/*.sh; do
                        [[ -f "$test_file" ]] || continue
                        local test_name
                        test_name=$(basename "$test_file" .sh)
                        if [[ "$test_name" == *"$test_filter"* ]]; then
                            found_match=true
                            break
                        fi
                    done
                    if [[ "$found_match" == "true" ]]; then
                        run_test_category "$tool" "$category" "$test_filter"
                    fi
                fi
            done
        done
        
        # Also run scenario tests
        if [[ -d "$TEST_DIR/integration/scenarios" ]]; then
            for scenario_file in "$TEST_DIR/integration/scenarios"/*.sh; do
                [[ -f "$scenario_file" ]] || continue
                if [[ -z "$test_filter" || "$(basename "$scenario_file")" == *"$test_filter"* ]]; then
                    run_test_file "$scenario_file"
                fi
            done
        fi
    fi
}

# Run all tests
run_all_tests() {
    echo -e "${BLUE}Running all integration tests...${NC}"
    
    for tool_dir in "$TEST_DIR/integration"/*; do
        [[ -d "$tool_dir" ]] || continue
        local tool
        tool=$(basename "$tool_dir")
        run_tool_tests "$tool"
    done
}

# Print test summary
print_summary() {
    echo
    echo -e "${BLUE}Test Summary:${NC}"
    echo -e "  Total:   $TOTAL_TESTS"
    echo -e "  ${GREEN}Passed:  $PASSED_TESTS${NC}"
    echo -e "  ${RED}Failed:  $FAILED_TESTS${NC}"
    echo -e "  ${YELLOW}Skipped: $SKIPPED_TESTS${NC}"
    
    if [[ $FAILED_TESTS -gt 0 ]]; then
        echo -e "\n${RED}Some tests failed!${NC}"
        return 1
    else
        echo -e "\n${GREEN}All tests passed!${NC}"
        return 0
    fi
}

# Main execution
# List available tests
list_tests() {
    local tool_filter="$1"
    local test_filter="$2"
    
    echo -e "${BLUE}Available Tests:${NC}"
    echo
    
    if [[ -n "$tool_filter" && "$tool_filter" != "all" ]]; then
        list_tool_tests "$tool_filter" "$test_filter"
    else
        for tool_dir in "$TEST_DIR/integration"/*; do
            [[ -d "$tool_dir" ]] || continue
            local tool
            tool=$(basename "$tool_dir")
            list_tool_tests "$tool" "$test_filter"
        done
        
        # List scenario tests
        if [[ -d "$TEST_DIR/integration/scenarios" ]]; then
            echo -e "${YELLOW}Scenarios:${NC}"
            for scenario_file in "$TEST_DIR/integration/scenarios"/*.sh; do
                [[ -f "$scenario_file" ]] || continue
                local scenario_name
                scenario_name=$(basename "$scenario_file" .sh)
                if [[ -z "$test_filter" || "$scenario_name" == *"$test_filter"* ]]; then
                    echo "  - $scenario_name"
                fi
            done
            echo
        fi
    fi
}

list_tool_tests() {
    local tool="$1"
    local test_filter="$2"
    
    if [[ ! -d "$TEST_DIR/integration/$tool" ]]; then
        return
    fi
    
    echo -e "${YELLOW}$tool:${NC}"
    for category_dir in "$TEST_DIR/integration/$tool"/*; do
        [[ -d "$category_dir" ]] || continue
        local category
        category=$(basename "$category_dir")
        
        if [[ -z "$test_filter" || "$category" == *"$test_filter"* ]]; then
            echo "  $category:"
            for test_file in "$category_dir"/*.sh; do
                [[ -f "$test_file" ]] || continue
                local test_name
                test_name=$(basename "$test_file" .sh)
                echo "    - $test_name"
            done
        fi
    done
    echo
}

# Main function
main() {
    local tool_filter=""
    local test_filter=""
    local list_only=false
    local verbose=false
    
    while [[ $# -gt 0 ]]; do
        case $1 in
            --tool)
                tool_filter="$2"
                shift 2
                ;;
            --test)
                test_filter="$2"
                shift 2
                ;;
            --list)
                list_only=true
                shift
                ;;
            --verbose|-v)
                verbose=true
                shift
                ;;
            -h|--help)
                echo "Test Runner for llmcmd and llmsh"
                echo "Usage: $0 [OPTIONS]"
                echo "  --tool TOOL     Run tests for specific tool (llmcmd, llmsh, all)"
                echo "  --test PATTERN  Run tests matching pattern"
                echo "  --list          List available tests"
                echo "  --verbose, -v   Enable verbose output"
                echo "  -h, --help      Show this help"
                exit 0
                ;;
            *)
                # Handle old-style positional arguments for backwards compatibility
                if [[ -z "$tool_filter" ]]; then
                    tool_filter="$1"
                elif [[ -z "$test_filter" ]]; then
                    test_filter="$1"
                fi
                shift
                ;;
        esac
    done
    
    if [ "$verbose" = true ]; then
        export TEST_VERBOSE=1
    fi
    
    trap cleanup_test_env EXIT
    setup_test_env
    
    if [ "$list_only" = true ]; then
        list_tests "$tool_filter" "$test_filter"
        exit 0
    fi
    
    run_tool_tests "$tool_filter" "$test_filter"
    print_summary
}

# Run main function with all arguments
main "$@"
