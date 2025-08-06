#!/bin/bash
# Test: Data migration scenario
# Purpose: Real-world data format conversion and validation

TEST_NAME="Data Migration Scenario"
TEST_TYPE="scenario"
TIMEOUT=45

# Source test framework
source "$(dirname "$0")/../../framework/fsproxy_helpers.sh"

test_function() {
    log_info "Testing real-world data migration scenario"
    
    # Create legacy data format
    cat > "$TEST_DATA/legacy_customers.dat" << 'EOF'
CUST001|John Smith|john.smith@email.com|555-1234|2020-01-15|ACTIVE
CUST002|Jane Doe|jane.doe@email.com|555-5678|2019-06-20|ACTIVE
CUST003|Bob Wilson|bob.wilson@email.com|555-9012|2021-03-10|INACTIVE
CUST004|Alice Brown|alice.brown@email.com|555-3456|2018-12-05|ACTIVE
CUST005|Charlie Davis|charlie.davis@email.com|555-7890|2022-08-18|SUSPENDED
CUST006|Eva Martinez|eva.martinez@email.com|555-2468|2020-11-22|ACTIVE
EOF
    
    # Create migration requirements
    cat > "$TEST_DATA/migration_spec.txt" << 'EOF'
We need to migrate our legacy customer database to a new JSON format for our modern system.

Legacy format: CUSTOMER_ID|FULL_NAME|EMAIL|PHONE|JOIN_DATE|STATUS

Required transformations:
1. Convert pipe-delimited data to JSON format
2. Split FULL_NAME into first_name and last_name
3. Validate email addresses (must contain @ and .)
4. Convert phone numbers to standard format (XXX-XXXX)
5. Convert dates from YYYY-MM-DD to ISO format
6. Normalize status values (ACTIVE->active, INACTIVE->inactive, SUSPENDED->suspended)
7. Add a migration timestamp to each record

Output requirements:
- migrated_customers.json: Array of customer objects in new format
- migration_report.txt: Summary including:
  * Total records processed
  * Number of successful conversions
  * Any validation errors or warnings
  * Data quality issues found
  * Migration completion status

Ensure data integrity throughout the migration process.
EOF
    
    # Execute migration scenario
    if timeout 45 "$LLMCMD_BIN" \
        -i "$TEST_DATA/legacy_customers.dat" \
        -i "$TEST_DATA/migration_spec.txt" \
        -o "$TEST_DATA/migrated_customers.json" \
        -o "$TEST_DATA/migration_report.txt" \
        "$TEST_DATA/migration_spec.txt" > "$LOG_FILE" 2>&1; then
        
        # Verify migration results
        if [ -f "$TEST_DATA/migrated_customers.json" ] && [ -f "$TEST_DATA/migration_report.txt" ]; then
            
            # Check JSON format
            if grep -q -E '\[\s*\{|\}\s*\]' "$TEST_DATA/migrated_customers.json" && \
               grep -q '"first_name"' "$TEST_DATA/migrated_customers.json" && \
               grep -q '"email"' "$TEST_DATA/migrated_customers.json"; then
                
                # Check migration report
                if grep -q -i "total.*record\|processed" "$TEST_DATA/migration_report.txt" && \
                   (grep -q -i "6.*record\|successful" "$TEST_DATA/migration_report.txt" || \
                    grep -q -i "migration.*complete" "$TEST_DATA/migration_report.txt"); then
                    log_success "Data migration scenario completed successfully"
                    return 0
                else
                    log_warning "Migration completed but report may be incomplete"
                    cat "$TEST_DATA/migration_report.txt" >> "$LOG_FILE" 2>/dev/null
                    return 1
                fi
            else
                log_error "JSON format conversion failed or incomplete"
                cat "$TEST_DATA/migrated_customers.json" >> "$LOG_FILE" 2>/dev/null
                return 1
            fi
        else
            log_error "Migration scenario output files not created"
            return 1
        fi
    else
        log_error "Data migration scenario failed"
        return 1
    fi
}

# Execute test
main_test_wrapper "$@"
