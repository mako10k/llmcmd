#!/bin/bash
# Test: Multi-step text analysis pipeline
# Purpose: Verify complex text processing through multiple stages

TEST_NAME="Text Analysis Pipeline"
TEST_TYPE="pipeline"
TIMEOUT=30

# Source test framework
source "$(dirname "$0")/../../framework/fsproxy_helpers.sh"

test_function() {
    log_info "Testing multi-step text analysis pipeline"
    
    # Create source text data
    cat > "$TEST_DATA/document.txt" << 'EOF'
The quick brown fox jumps over the lazy dog. This sentence contains every letter of the alphabet.
Performance testing is critical for software quality. Performance issues can impact user experience significantly.
Data processing pipelines require careful design and testing. Data validation is essential for reliable results.
Quality assurance processes help ensure software reliability. Quality metrics should be tracked continuously.
Testing automation reduces manual effort and improves efficiency. Testing strategies should be comprehensive.
EOF
    
    # Create pipeline instruction
    cat > "$TEST_DATA/instruction.txt" << 'EOF'
Please analyze the document.txt file through this pipeline:

1. Read the document
2. Count total words and lines
3. Find the most frequently used words (excluding common words like "the", "is", "and")
4. Extract all sentences containing the word "testing" or "quality"
5. Create an analysis report in analysis.txt with:
   - Document statistics (word count, line count)
   - Top 5 most frequent meaningful words with their counts
   - All sentences containing "testing" or "quality"
   - Summary section with key findings

Use text processing commands to accomplish this analysis.
EOF
    
    # Execute analysis pipeline
    if timeout 30 "$LLMCMD_BIN" \
        -i "$TEST_DATA/document.txt" \
        -i "$TEST_DATA/instruction.txt" \
        -o "$TEST_DATA/analysis.txt" \
        "$TEST_DATA/instruction.txt" > "$LOG_FILE" 2>&1; then
        
        # Verify analysis results
        if [ -f "$TEST_DATA/analysis.txt" ] && \
           grep -q -i "word.*count\|statistics" "$TEST_DATA/analysis.txt" && \
           grep -q -i "testing\|quality" "$TEST_DATA/analysis.txt"; then
            
            # Check if analysis contains expected elements
            if grep -q -i "performance\|data\|testing" "$TEST_DATA/analysis.txt"; then
                log_success "Text analysis pipeline completed successfully"
                return 0
            else
                log_warning "Pipeline completed but analysis may be incomplete"
                cat "$TEST_DATA/analysis.txt" >> "$LOG_FILE" 2>/dev/null
                return 1
            fi
        else
            log_error "Analysis pipeline output missing expected content"
            cat "$TEST_DATA/analysis.txt" >> "$LOG_FILE" 2>/dev/null
            return 1
        fi
    else
        log_error "Text analysis pipeline failed"
        return 1
    fi
}

# Execute test
main_test_wrapper "$@"
