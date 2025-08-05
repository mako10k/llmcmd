#!/bin/bash

# llmsh Pipeline Tests

test_simple_pipes() {
    setup_test
    
    # Test simple pipe
    run_llmsh -c 'echo hello | grep hello'
    assert_success "Simple pipe should succeed"
    assert_output_contains "hello" "Pipe should output 'hello'"
    
    # Test pipe with multiple commands
    run_llmsh -c 'echo -e "apple\nbanana\napricot" | grep "a"'
    assert_success "Multi-match pipe should succeed"
    
    teardown_test
}

test_complex_pipes() {
    setup_test
    
    # Test complex pipeline
    run_llmsh -c 'echo -e "line1\nline2\nline3" | grep line | head -2'
    assert_success "Complex pipeline should succeed"
    
    # Test sort pipeline
    run_llmsh -c 'echo -e "c\nb\na" | sort'
    assert_success "Sort pipeline should succeed"
    
    teardown_test
}

test_pipe_error_propagation() {
    setup_test
    
    # Test error in first command
    run_llmsh -c 'cat nonexistent.txt | grep something'
    assert_failure "Pipeline should fail if first command fails"
    
    teardown_test
}
