#!/bin/bash

# Performance and Edge Case Scenarios

# Main test function for this file
test_performance_edge_cases() {
    test_large_file_processing
    test_concurrent_execution
    test_memory_usage_scenario
}

test_large_file_processing() {
    setup_test
    
    local large_file
    large_file=$(create_temp_file)
    
    # Create a moderately large file for testing
    for i in {1..1000}; do
        echo "Line $i with some test data" >> "$large_file"
    done
    
    # Test large file processing with llmsh
    local start_time end_time
    start_time=$(date +%s)
    run_llmsh -i "$large_file" -c 'cat | wc -l'
    end_time=$(date +%s)
    
    assert_success "Large file processing should succeed"
    assert_output_contains "1000" "Should count 1000 lines"
    
    local duration=$((end_time - start_time))
    if [ $duration -gt 30 ]; then
        log_warning "Large file processing took $duration seconds (>30s)"
    fi
    
    teardown_test
}

test_concurrent_execution() {
    setup_test
    
    local temp_dir
    temp_dir=$(mktemp -d)
    
    # Test concurrent llmsh execution
    for i in {1..3}; do
        echo "data$i" > "$temp_dir/input$i.txt"
    done
    
    # Run multiple llmsh processes concurrently
    run_llmsh -i "$temp_dir/input1.txt" -o "$temp_dir/output1.txt" -c 'cat | tr a-z A-Z' &
    local pid1=$!
    run_llmsh -i "$temp_dir/input2.txt" -o "$temp_dir/output2.txt" -c 'cat | tr a-z A-Z' &
    local pid2=$!
    run_llmsh -i "$temp_dir/input3.txt" -o "$temp_dir/output3.txt" -c 'cat | tr a-z A-Z' &
    local pid3=$!
    
    # Wait for all processes
    wait $pid1 $pid2 $pid3
    
    # Check results
    for i in {1..3}; do
        assert_file_exists "$temp_dir/output$i.txt" "Output file $i should exist"
        local content
        content=$(cat "$temp_dir/output$i.txt")
        assert_string_contains "$content" "DATA$i" "Should contain transformed data"
    done
    
    rm -rf "$temp_dir"
    teardown_test
}

test_memory_usage_scenario() {
    setup_test
    
    # Test memory usage with repeated operations
    for i in {1..50}; do
        run_llmsh -c "echo 'test iteration $i'" > /dev/null
        if [ $? -ne 0 ]; then
            log_error "Memory test failed at iteration $i"
            break
        fi
    done
    
    assert_success "Memory usage test should complete successfully"
    
    teardown_test
}
