#!/bin/bash

# Real-world Usage Scenarios

# Main test function for this file
test_real_world_usage() {
    test_data_processing_scenario
    test_log_analysis_scenario
    test_file_transformation_scenario
}

test_data_processing_scenario() {
    setup_test
    
    local input_file output_file
    input_file=$(create_temp_file)
    output_file=$(create_temp_file)
    
    # Create sample CSV data
    cat > "$input_file" << 'EOF'
name,age,city
Alice,25,Tokyo
Bob,30,Osaka
Charlie,35,Tokyo
Diana,28,Kyoto
EOF
    
    # Test data processing scenario with llmcmd
    run_llmcmd --config "$TEST_CONFIG" "Process the CSV file '$input_file': extract lines for Tokyo residents and save to '$output_file'"
    assert_success "Data processing scenario should succeed"
    
    assert_file_exists "$output_file" "Output file should be created"
    local content
    content=$(cat "$output_file")
    assert_string_contains "$content" "Tokyo" "Should contain Tokyo residents"
    assert_string_contains "$content" "Alice" "Should contain Alice"
    
    teardown_test
}

test_log_analysis_scenario() {
    setup_test
    
    local log_file
    log_file=$(create_temp_file)
    
    # Create sample log data
    cat > "$log_file" << 'EOF'
2024-01-01 10:00:00 INFO User login successful
2024-01-01 10:05:00 ERROR Database connection failed
2024-01-01 10:06:00 INFO Retry successful
2024-01-01 10:10:00 ERROR Authentication failed
2024-01-01 10:15:00 INFO User logout
EOF
    
    # Test log analysis with llmsh
    run_llmsh -i "$log_file" -c 'cat | grep ERROR | wc -l'
    assert_success "Log analysis should succeed"
    assert_output_contains "2" "Should count 2 error lines"
    
    teardown_test
}

test_file_transformation_scenario() {
    setup_test
    
    local input_file output_file
    input_file=$(create_temp_file)
    output_file=$(create_temp_file)
    
    echo -e "apple\nbanana\ncherry\napple\nbanana" > "$input_file"
    
    # Test file transformation scenario
    run_llmcmd --config "$TEST_CONFIG" "Transform file '$input_file': remove duplicates, sort alphabetically, convert to uppercase, and save to '$output_file'"
    assert_success "File transformation should succeed"
    
    assert_file_exists "$output_file" "Transformed file should exist"
    local content
    content=$(cat "$output_file")
    assert_string_contains "$content" "APPLE" "Should contain uppercase sorted unique content"
    
    teardown_test
}
