#!/bin/bash

# llmsh Advanced Pipeline Tests

test_file_pipeline() {
    setup_test
    
    # Create test file
    local test_file
    test_file=$(create_temp_file)
    echo -e "red\ngreen\nblue\nred\ngreen" > "$test_file"
    
    # Test file to pipeline
    run_llmsh -c "cat '$test_file' | sort | uniq"
    assert_success "File pipeline should succeed"
    assert_output_contains "blue" "Should contain sorted unique colors"
    
    teardown_test
}

test_multiple_filters() {
    setup_test
    
    # Test multiple filter commands
    run_llmsh -c 'echo -e "1\n2\n3\n4\n5" | grep -E "[2-4]" | head -2'
    assert_success "Multiple filters should succeed"
    
    teardown_test
}

test_pipe_with_input_output() {
    setup_test
    
    local input_file output_file
    input_file=$(create_temp_file)
    output_file=$(create_temp_file)
    
    echo -e "apple\nbanana\ncherry" > "$input_file"
    
    # Test pipeline with input and output redirection
    run_llmsh -i "$input_file" -o "$output_file" -c 'cat | sort -r'
    assert_success "Pipeline with I/O redirection should succeed"
    
    assert_file_exists "$output_file" "Output file should be created"
    local content
    content=$(cat "$output_file")
    assert_string_contains "$content" "cherry" "Should contain sorted content"
    
    teardown_test
}
