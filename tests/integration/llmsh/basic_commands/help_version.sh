#!/bin/bash

# llmsh Basic Commands Test

test_help_version() {
    setup_test
    
    # Test --help
    run_llmsh --help
    assert_success "llmsh --help should succeed"
    assert_output_contains "Usage:" "Help should contain usage information"
    assert_output_contains "Options:" "Help should contain options"
    
    # Test --version
    run_llmsh --version
    assert_success "llmsh --version should succeed"
    assert_output_contains "llmsh version" "Version should contain 'llmsh version'"
    
    teardown_test
}

test_command_execution() {
    setup_test
    
    # Test simple echo command
    run_llmsh -c 'echo hello world'
    assert_success "Echo command should succeed"
    assert_output_contains "hello world" "Echo should output 'hello world'"
    
    # Test script from stdin
    echo 'echo "from stdin"' | run_llmsh
    assert_success "Script from stdin should succeed"
    assert_output_contains "from stdin" "Should output 'from stdin'"
    
    teardown_test
}

test_script_files() {
    setup_test
    
    # Create test script
    local script_file
    script_file=$(create_temp_file 'echo "from script file"' "test.llmsh")
    
    # Test script file execution
    run_llmsh "$script_file"
    assert_success "Script file execution should succeed"
    assert_output_contains "from script file" "Should execute script file content"
    
    teardown_test
}

test_option_validation() {
    setup_test
    
    # Test -i option without --virtual (should fail)
    run_llmsh -i nonexistent.txt -c 'echo hello'
    assert_failure "Should fail when using -i without --virtual"
    assert_error_contains "require --virtual" "Error should mention --virtual requirement"
    
    # Test -o option without --virtual (should fail)
    run_llmsh -o output.txt -c 'echo hello'
    assert_failure "Should fail when using -o without --virtual"
    assert_error_contains "require --virtual" "Error should mention --virtual requirement"
    
    # Test invalid option
    run_llmsh --invalid-option
    assert_failure "Should fail with invalid option"
    assert_error_contains "unknown option" "Error should mention unknown option"
    
    teardown_test
}
