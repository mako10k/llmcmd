#!/bin/bash

# llmsh Virtual Mode Tests

test_virtual_mode_basic() {
    setup_test
    
    # Test virtual mode activation
    run_llmsh --virtual -c 'echo hello'
    assert_success "Virtual mode should work with basic commands"
    assert_output_contains "hello" "Should output hello in virtual mode"
    
    teardown_test
}

test_virtual_mode_file_restrictions() {
    setup_test
    
    # Test file access restriction in virtual mode
    run_llmsh --virtual -c 'cat /etc/passwd'
    assert_failure "Virtual mode should restrict file access"
    
    # Test directory listing restriction
    run_llmsh --virtual -c 'ls /'
    assert_failure "Virtual mode should restrict directory access"
    
    teardown_test
}

test_virtual_mode_with_input_output() {
    setup_test
    
    local input_file output_file
    input_file=$(create_temp_file)
    output_file=$(create_temp_file)
    
    echo "test data" > "$input_file"
    
    # Test virtual mode with -i and -o options
    run_llmsh --virtual -i "$input_file" -o "$output_file" -c 'cat | tr a-z A-Z'
    assert_success "Virtual mode should work with -i/-o options"
    
    assert_file_exists "$output_file" "Output file should be created"
    local content
    content=$(cat "$output_file")
    assert_string_contains "$content" "TEST DATA" "Should contain uppercase output"
    
    teardown_test
}

test_virtual_mode_command_restrictions() {
    setup_test
    
    # Test restricted commands in virtual mode
    run_llmsh --virtual -c 'rm -rf /'
    assert_failure "Virtual mode should block dangerous commands"
    
    # Test network commands restriction
    run_llmsh --virtual -c 'wget http://example.com'
    assert_failure "Virtual mode should block network commands"
    
    teardown_test
}
