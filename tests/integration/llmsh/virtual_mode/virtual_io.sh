#!/bin/bash

# llmsh Virtual Mode I/O Tests

test_virtual_input_output_validation() {
    setup_test
    
    local input_file output_file nonexistent_file
    input_file=$(create_temp_file)
    output_file=$(create_temp_file)
    nonexistent_file="/tmp/nonexistent_$(date +%s).txt"
    
    echo "input data" > "$input_file"
    
    # Test valid input file
    run_llmsh --virtual -i "$input_file" -c 'cat'
    assert_success "Virtual mode should accept valid input file"
    assert_output_contains "input data" "Should read input file content"
    
    # Test nonexistent input file
    run_llmsh --virtual -i "$nonexistent_file" -c 'cat'
    assert_failure "Virtual mode should reject nonexistent input file"
    
    # Test output file creation
    run_llmsh --virtual -i "$input_file" -o "$output_file" -c 'cat'
    assert_success "Virtual mode should create output file"
    assert_file_exists "$output_file" "Output file should be created"
    
    teardown_test
}

test_virtual_mode_without_io() {
    setup_test
    
    # Test virtual mode without -i/-o (should fail for most operations)
    run_llmsh --virtual -c 'cat'
    assert_failure "Virtual mode without -i should fail for cat"
    
    # Test commands that work without input
    run_llmsh --virtual -c 'echo hello world'
    assert_success "Virtual mode should work with echo"
    
    teardown_test
}

test_virtual_mode_pipeline_with_io() {
    setup_test
    
    local input_file output_file
    input_file=$(create_temp_file)
    output_file=$(create_temp_file)
    
    echo -e "apple\nbanana\ncherry" > "$input_file"
    
    # Test pipeline in virtual mode with I/O
    run_llmsh --virtual -i "$input_file" -o "$output_file" -c 'cat | sort -r | head -2'
    assert_success "Virtual mode pipeline with I/O should succeed"
    
    assert_file_exists "$output_file" "Output file should be created"
    local content
    content=$(cat "$output_file")
    assert_string_contains "$content" "cherry" "Should contain sorted output"
    
    teardown_test
}
