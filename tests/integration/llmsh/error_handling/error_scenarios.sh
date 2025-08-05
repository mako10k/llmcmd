#!/bin/bash

# llmsh Error Handling Tests
## USER COMMENT: need --virtual and not --virtual for consistency with other tests
## FIXED: Added both --virtual and normal mode tests for consistency

# Main test function for this file
test_error_scenarios() {
    test_invalid_command_error
    test_syntax_error_handling
    test_permission_error
    test_file_not_found
    test_pipe_error_propagation
}

test_invalid_command_error() {
    setup_test
    
    # Test invalid command in normal mode
    run_llmsh -c 'nonexistentcommand123'
    assert_failure "Invalid command should fail in normal mode"
    assert_output_contains "command not found\|not found" "Should show command not found error"
    
    # Test invalid command in virtual mode
    run_llmsh --virtual -c 'nonexistentcommand123'
    assert_failure "Invalid command should fail in virtual mode"
    assert_output_contains "command not found\|not found" "Should show command not found error in virtual mode"
    
    teardown_test
}

test_syntax_error_handling() {
    setup_test
    
    # Test syntax errors in normal mode
    run_llmsh -c 'echo hello | |'
    assert_failure "Syntax error should fail in normal mode"
    
    # Test unclosed quotes in normal mode
    run_llmsh -c 'echo "unclosed quote'
    assert_failure "Unclosed quote should fail in normal mode"
    
    # Test syntax errors in virtual mode
    run_llmsh --virtual -c 'echo hello | |'
    assert_failure "Syntax error should fail in virtual mode"
    
    # Test unclosed quotes in virtual mode
    run_llmsh --virtual -c 'echo "unclosed quote'
    assert_failure "Unclosed quote should fail in virtual mode"
    
    teardown_test
}

test_permission_error() {
    setup_test
    
    # Test permission denied (if running as non-root)
    if [ "$(id -u)" -ne 0 ]; then
        # Running as non-root user - test permission denial
        run_llmsh -c 'cat /etc/shadow'
        assert_failure "Permission denied should fail"
        
        # Test same in virtual mode
        run_llmsh --virtual -c 'cat /etc/shadow'
        assert_failure "Permission denied should fail in virtual mode"
    else
        # Running as root - test alternative permission scenarios
        echo "⚠️  Running as root - testing alternative permission scenarios"
        
        # Create a test file with restricted permissions
        local restricted_file
        restricted_file=$(create_temp_file)
        chmod 000 "$restricted_file"
        
        # Test access to restricted file we created
        run_llmsh -c "cat '$restricted_file'"
        assert_failure "Access to 000 permission file should fail"
        
        # Test same in virtual mode
        run_llmsh --virtual -c "cat '$restricted_file'"
        assert_failure "Access to 000 permission file should fail in virtual mode"
        
        # Clean up
        chmod 644 "$restricted_file"
    fi
    ## USER COMMENT: This is silently skipped if running as root
    ## FIXED: Added explicit handling and alternative tests for root execution
    
    teardown_test
}

test_file_not_found() {
    setup_test
    
    # Test file not found in normal mode
    run_llmsh -c 'cat /nonexistent/path/file.txt'
    assert_failure "File not found should fail in normal mode"
    assert_output_contains "No such file\|not found" "Should show file not found error"
    
    # Test file not found in virtual mode
    run_llmsh --virtual -c 'cat /nonexistent/path/file.txt'
    assert_failure "File not found should fail in virtual mode"
    assert_output_contains "No such file\|not found" "Should show file not found error in virtual mode"
    
    teardown_test
}

test_pipe_error_propagation() {
    setup_test
    
    # Test error propagation in pipes - normal mode
    run_llmsh -c 'cat /nonexistent/file.txt | grep something'
    assert_failure "Pipe with failing first command should fail in normal mode"
    
    # Test error propagation in pipes - virtual mode
    run_llmsh --virtual -c 'cat /nonexistent/file.txt | grep something'
    assert_failure "Pipe with failing first command should fail in virtual mode"
    
    teardown_test
}
