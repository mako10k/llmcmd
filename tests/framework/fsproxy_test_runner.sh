#!/bin/bash

# FSProxy Protocol Test Runner
# Comprehensive test execution with categorized reporting

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_ROOT="$(dirname "$SCRIPT_DIR")"
RESULTS_DIR="$TEST_ROOT/results/$(date +%Y%m%d_%H%M%S)"

# Test Categories
BINARY_TESTS=(
    "integration/llmcmd"
    "security/llmcmd"
    "integration/llmsh_virtual"
    "security/llmsh_virtual"
    "integration/llmsh_real"
)

UNIT_TESTS=(
    "unit/fsproxy_protocol"
    "unit/llm_integration"
    "unit/resource_management"
)

PIPELINE_TESTS=(
    "integration/pipelines"
    "integration/redirection"
    "integration/vfs_streams"
)

SCENARIO_TESTS=(
    "scenarios/workflows"
    "scenarios/error_recovery"
    "scenarios/performance"
    "scenarios/security"
)

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[PASS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[FAIL]${NC} $1"
}

# Test execution functions
run_test_file() {
    local test_file="$1"
    local test_name
    test_name=$(basename "$test_file" .sh)
    
    log_info "Running test: $test_name"
    
    local start_time
    start_time=$(date +%s)
    
    local result_file="$RESULTS_DIR/${test_name}.result"
    local log_file="$RESULTS_DIR/${test_name}.log"
    
    if timeout 300 "$test_file" > "$log_file" 2>&1; then
        local end_time
        end_time=$(date +%s)
        local duration=$((end_time - start_time))
        
        echo "PASSED $duration" > "$result_file"
        log_success "$test_name (${duration}s)"
        return 0
    else
        local exit_code=$?
        local end_time
        end_time=$(date +%s)
        local duration=$((end_time - start_time))
        
        if [[ $exit_code -eq 124 ]]; then
            echo "TIMEOUT $duration" > "$result_file"
            log_error "$test_name (TIMEOUT after ${duration}s)"
        else
            echo "FAILED $duration $exit_code" > "$result_file"
            log_error "$test_name (EXIT CODE: $exit_code after ${duration}s)"
        fi
        
        # Show last few lines of error log
        log_warning "Last 5 lines of error log:"
        tail -5 "$log_file" | sed 's/^/  /'
        
        return $exit_code
    fi
}

run_test_directory() {
    local test_dir="$1"
    
    if [[ ! -d "$test_dir" ]]; then
        log_warning "Test directory not found: $test_dir"
        return 0
    fi
    
    log_info "Running tests in: $test_dir"
    
    local test_count=0
    local passed_count=0
    local failed_count=0
    
    # Find all .sh files and run them
    while IFS= read -r -d '' test_file; do
        if [[ -x "$test_file" ]]; then
            ((test_count++))
            if run_test_file "$test_file"; then
                ((passed_count++))
            else
                ((failed_count++))
            fi
        else
            log_warning "Test file not executable: $test_file"
        fi
    done < <(find "$test_dir" -name "*.sh" -type f -print0 | sort -z)
    
    log_info "Directory summary: $passed_count passed, $failed_count failed, $test_count total"
    
    return $failed_count
}

run_category_tests() {
    local category="$1"
    shift
    local test_dirs=("$@")
    
    echo ""
    echo "======================================="
    echo "Running $category Tests"
    echo "======================================="
    
    local total_failures=0
    
    for test_dir in "${test_dirs[@]}"; do
        local full_path="$TEST_ROOT/$test_dir"
        if ! run_test_directory "$full_path"; then
            ((total_failures += $?))
        fi
    done
    
    return $total_failures
}

# Report generation
generate_summary_report() {
    local report_file="$RESULTS_DIR/summary.txt"
    local html_report="$RESULTS_DIR/report.html"
    
    echo "FSProxy Protocol Test Summary" > "$report_file"
    echo "Generated: $(date)" >> "$report_file"
    echo "===============================" >> "$report_file"
    echo "" >> "$report_file"
    
    local total_tests=0
    local total_passed=0
    local total_failed=0
    local total_timeout=0
    
    for result_file in "$RESULTS_DIR"/*.result; do
        if [[ -f "$result_file" ]]; then
            local test_name
            test_name=$(basename "$result_file" .result)
            local status
            status=$(cut -d' ' -f1 "$result_file")
            local duration
            duration=$(cut -d' ' -f2 "$result_file")
            
            ((total_tests++))
            
            case "$status" in
                "PASSED")
                    ((total_passed++))
                    echo "✓ $test_name (${duration}s)" >> "$report_file"
                    ;;
                "FAILED")
                    ((total_failed++))
                    local exit_code
                    exit_code=$(cut -d' ' -f3 "$result_file")
                    echo "✗ $test_name (${duration}s, exit: $exit_code)" >> "$report_file"
                    ;;
                "TIMEOUT")
                    ((total_timeout++))
                    ((total_failed++))
                    echo "⏰ $test_name (TIMEOUT after ${duration}s)" >> "$report_file"
                    ;;
            esac
        fi
    done
    
    echo "" >> "$report_file"
    echo "Summary:" >> "$report_file"
    echo "  Total tests: $total_tests" >> "$report_file"
    echo "  Passed: $total_passed" >> "$report_file"
    echo "  Failed: $total_failed" >> "$report_file"
    echo "  Timeouts: $total_timeout" >> "$report_file"
    echo "  Success rate: $(( total_tests > 0 ? (total_passed * 100) / total_tests : 0 ))%" >> "$report_file"
    
    # Display summary
    echo ""
    echo "======================================="
    echo "TEST SUMMARY"
    echo "======================================="
    cat "$report_file"
    
    echo ""
    echo "Detailed results saved to: $RESULTS_DIR"
    
    return $total_failed
}

# Build check
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    # Check if binaries exist
    local llmcmd_binary="$TEST_ROOT/../bin/llmcmd"
    local llmsh_binary="$TEST_ROOT/../bin/llmsh"
    
    if [[ ! -f "$llmcmd_binary" ]]; then
        log_error "llmcmd binary not found: $llmcmd_binary"
        log_info "Run: go build -o bin/llmcmd ./cmd/llmcmd"
        exit 1
    fi
    
    if [[ ! -f "$llmsh_binary" ]]; then
        log_warning "llmsh binary not found: $llmsh_binary (some tests may be skipped)"
    fi
    
    if [[ ! -x "$llmcmd_binary" ]]; then
        log_error "llmcmd binary not executable: $llmcmd_binary"
        exit 1
    fi
    
    log_success "Prerequisites check passed"
}

# Main execution
main() {
    local category="${1:-all}"
    
    mkdir -p "$RESULTS_DIR"
    
    echo "FSProxy Protocol Comprehensive Test Suite"
    echo "Results will be saved to: $RESULTS_DIR"
    echo ""
    
    check_prerequisites
    
    local total_failures=0
    
    case "$category" in
        "binary"|"all")
            if ! run_category_tests "BINARY" "${BINARY_TESTS[@]}"; then
                ((total_failures += $?))
            fi
            ;;&
        "unit"|"all")
            if ! run_category_tests "UNIT" "${UNIT_TESTS[@]}"; then
                ((total_failures += $?))
            fi
            ;;&
        "pipeline"|"all")
            if ! run_category_tests "PIPELINE" "${PIPELINE_TESTS[@]}"; then
                ((total_failures += $?))
            fi
            ;;&
        "scenario"|"all")
            if ! run_category_tests "SCENARIO" "${SCENARIO_TESTS[@]}"; then
                ((total_failures += $?))
            fi
            ;;&
        "all")
            ;;
        *)
            log_error "Unknown test category: $category"
            log_info "Available categories: binary, unit, pipeline, scenario, all"
            exit 1
            ;;
    esac
    
    # Generate final report
    generate_summary_report
    
    if [[ $total_failures -eq 0 ]]; then
        log_success "All tests passed!"
        exit 0
    else
        log_error "$total_failures test(s) failed"
        exit 1
    fi
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
