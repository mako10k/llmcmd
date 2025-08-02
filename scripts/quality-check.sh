#!/bin/bash
# llmcmd Quality Check Dashboard
# Comprehensive code quality analysis for Sprint 2

set -e

PROJECT_ROOT=$(dirname $(dirname $(realpath $0)))
REPORT_FILE="$PROJECT_ROOT/reports/quality-report-$(date +%Y%m%d-%H%M%S).md"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== llmcmd Quality Check Dashboard ===${NC}"
echo "Timestamp: $(date)"
echo "Report will be saved to: $REPORT_FILE"
echo

# Create reports directory
mkdir -p "$PROJECT_ROOT/reports"

# Initialize report
cat > "$REPORT_FILE" << EOF
# llmcmd Code Quality Report
Generated: $(date)

## Summary
EOF

echo -e "${YELLOW}1. Project Statistics${NC}"
echo "Analyzing project structure..."

# Count lines of code
TOTAL_LINES=$(find "$PROJECT_ROOT" -name "*.go" -not -path "*/vendor/*" -exec wc -l {} + | tail -1 | awk '{print $1}')
GO_FILES=$(find "$PROJECT_ROOT" -name "*.go" -not -path "*/vendor/*" | wc -l)

echo "  - Total Go files: $GO_FILES"
echo "  - Total lines of code: $TOTAL_LINES"

cat >> "$REPORT_FILE" << EOF

### Project Statistics
- **Total Go Files**: $GO_FILES
- **Total Lines of Code**: $TOTAL_LINES
- **Average Lines per File**: $((TOTAL_LINES / GO_FILES))

EOF

echo -e "${YELLOW}2. Build & Test Status${NC}"
echo "Running build tests..."

# Test build
cd "$PROJECT_ROOT"
if go build ./...; then
    BUILD_STATUS="✅ PASS"
    echo -e "  Build: ${GREEN}PASS${NC}"
else
    BUILD_STATUS="❌ FAIL"
    echo -e "  Build: ${RED}FAIL${NC}"
fi

# Test suite
if go test ./... > /tmp/test_output.txt 2>&1; then
    TEST_STATUS="✅ PASS"
    echo -e "  Tests: ${GREEN}PASS${NC}"
    TEST_COUNT=$(grep -c "=== RUN" /tmp/test_output.txt || echo "0")
else
    TEST_STATUS="❌ FAIL"
    echo -e "  Tests: ${RED}FAIL${NC}"
    TEST_COUNT="0"
fi

cat >> "$REPORT_FILE" << EOF
### Build & Test Status
- **Build Status**: $BUILD_STATUS
- **Test Status**: $TEST_STATUS
- **Test Count**: $TEST_COUNT tests

EOF

echo -e "${YELLOW}3. Code Complexity Analysis${NC}"
echo "Analyzing code complexity..."

# Find functions with high line count
echo "  - Analyzing function sizes..."
LARGE_FUNCTIONS=$(find "$PROJECT_ROOT" -name "*.go" -not -path "*/vendor/*" -exec grep -n "^func " {} + | \
    while IFS=: read -r file line func_def; do
        func_name=$(echo "$func_def" | sed 's/func //; s/(.*//; s/\*//')
        # Count lines until next function or end of file
        start_line=$(echo "$line")
        end_line=$(tail -n +$((start_line + 1)) "$file" | grep -n "^func \|^type \|^var \|^const \|^package " | head -1 | cut -d: -f1)
        if [ -n "$end_line" ]; then
            func_lines=$((end_line))
        else
            func_lines=$(($(wc -l < "$file") - start_line))
        fi
        if [ "$func_lines" -gt 50 ]; then
            echo "$file:$func_name:$func_lines"
        fi
    done | head -5)

# Check for files with high line count
LARGE_FILES=$(find "$PROJECT_ROOT" -name "*.go" -not -path "*/vendor/*" -exec wc -l {} + | \
    awk '$1 > 500 {print $2 ":" $1}' | head -5)

cat >> "$REPORT_FILE" << EOF
### Code Complexity Analysis

#### Large Files (>500 lines)
EOF

if [ -n "$LARGE_FILES" ]; then
    echo "$LARGE_FILES" | while read line; do
        echo "- $line" >> "$REPORT_FILE"
    done
    echo -e "  Large files found: ${YELLOW}$(echo "$LARGE_FILES" | wc -l)${NC}"
else
    echo "- None found ✅" >> "$REPORT_FILE"
    echo -e "  Large files: ${GREEN}None${NC}"
fi

cat >> "$REPORT_FILE" << EOF

#### Large Functions (>50 lines)
EOF

if [ -n "$LARGE_FUNCTIONS" ]; then
    echo "$LARGE_FUNCTIONS" | while read line; do
        echo "- $line" >> "$REPORT_FILE"
    done
    echo -e "  Large functions found: ${YELLOW}$(echo "$LARGE_FUNCTIONS" | wc -l)${NC}"
else
    echo "- None found ✅" >> "$REPORT_FILE"
    echo -e "  Large functions: ${GREEN}None${NC}"
fi

echo -e "${YELLOW}4. Error Handling Analysis${NC}"
echo "Checking error handling patterns..."

# Count error handling patterns
ERROR_CHECKS=$(find "$PROJECT_ROOT" -name "*.go" -not -path "*/vendor/*" -exec grep -c "if err != nil" {} + | \
    awk -F: 'BEGIN{sum=0} {sum+=$2} END{print sum}')

ERROR_RETURNS=$(find "$PROJECT_ROOT" -name "*.go" -not -path "*/vendor/*" -exec grep -c "return.*err" {} + | \
    awk -F: 'BEGIN{sum=0} {sum+=$2} END{print sum}')

PANIC_USAGE=$(find "$PROJECT_ROOT" -name "*.go" -not -path "*/vendor/*" -exec grep -c "panic(" {} + | \
    awk -F: 'BEGIN{sum=0} {sum+=$2} END{print sum}')

echo "  - Error checks: $ERROR_CHECKS"
echo "  - Error returns: $ERROR_RETURNS"
echo "  - Panic usage: $PANIC_USAGE"

cat >> "$REPORT_FILE" << EOF

### Error Handling Analysis
- **Error Checks (if err != nil)**: $ERROR_CHECKS
- **Error Returns**: $ERROR_RETURNS
- **Panic Usage**: $PANIC_USAGE $(if [ "$PANIC_USAGE" -eq 0 ]; then echo "✅"; else echo "⚠️"; fi)

EOF

echo -e "${YELLOW}5. Documentation Coverage${NC}"
echo "Analyzing documentation..."

# Count documented functions vs total functions
TOTAL_FUNCS=$(find "$PROJECT_ROOT" -name "*.go" -not -path "*/vendor/*" -exec grep -c "^func " {} + | \
    awk -F: 'BEGIN{sum=0} {sum+=$2} END{print sum}')

DOCUMENTED_FUNCS=$(find "$PROJECT_ROOT" -name "*.go" -not -path "*/vendor/*" -exec grep -B1 "^func " {} + | \
    grep -c "^// " 2>/dev/null || echo "0")

if [ "$TOTAL_FUNCS" -gt 0 ] && [ "$DOCUMENTED_FUNCS" -gt 0 ]; then
    DOC_PERCENTAGE=$((DOCUMENTED_FUNCS * 100 / TOTAL_FUNCS))
else
    DOC_PERCENTAGE=0
fi

echo "  - Total functions: $TOTAL_FUNCS"
echo "  - Documented functions: $DOCUMENTED_FUNCS"
echo "  - Documentation coverage: $DOC_PERCENTAGE%"

cat >> "$REPORT_FILE" << EOF
### Documentation Coverage
- **Total Functions**: $TOTAL_FUNCS
- **Documented Functions**: $DOCUMENTED_FUNCS
- **Coverage**: $DOC_PERCENTAGE% $(if [ "$DOC_PERCENTAGE" -gt 70 ]; then echo "✅"; elif [ "$DOC_PERCENTAGE" -gt 40 ]; then echo "⚠️"; else echo "❌"; fi)

EOF

echo -e "${YELLOW}6. Security & Best Practices${NC}"
echo "Checking security patterns..."

# Security checks
HARDCODED_SECRETS=$(find "$PROJECT_ROOT" -name "*.go" -not -path "*/vendor/*" -exec grep -i "password\|secret\|key\|token" {} + 2>/dev/null | \
    grep -v "// " | wc -l || echo "0")

SQL_INJECTION_RISK=$(find "$PROJECT_ROOT" -name "*.go" -not -path "*/vendor/*" -exec grep -c "fmt.Sprintf.*%.*sql\|fmt.Printf.*%.*sql" {} + 2>/dev/null | \
    awk -F: 'BEGIN{sum=0} {sum+=$2} END{print sum}' || echo "0")

cat >> "$REPORT_FILE" << EOF
### Security Analysis
- **Potential Hardcoded Secrets**: $HARDCODED_SECRETS $(if [ "$HARDCODED_SECRETS" -eq 0 ]; then echo "✅"; else echo "⚠️"; fi)
- **SQL Injection Risks**: $SQL_INJECTION_RISK $(if [ "$SQL_INJECTION_RISK" -eq 0 ]; then echo "✅"; else echo "⚠️"; fi)

EOF

echo "  - Hardcoded secrets check: $HARDCODED_SECRETS potential issues"
echo "  - SQL injection risks: $SQL_INJECTION_RISK potential issues"

echo -e "${YELLOW}7. Quality Score Calculation${NC}"

# Calculate overall quality score
SCORE=100
[ "$BUILD_STATUS" = "❌ FAIL" ] && SCORE=$((SCORE - 30))
[ "$TEST_STATUS" = "❌ FAIL" ] && SCORE=$((SCORE - 20))
[ -n "$DOC_PERCENTAGE" ] && [ "$DOC_PERCENTAGE" -lt 50 ] && SCORE=$((SCORE - 15))
[ -n "$PANIC_USAGE" ] && [ "$PANIC_USAGE" -gt 3 ] && SCORE=$((SCORE - 10))
[ -n "$HARDCODED_SECRETS" ] && [ "$HARDCODED_SECRETS" -gt 0 ] && SCORE=$((SCORE - 10))
[ -n "$LARGE_FILES" ] && SCORE=$((SCORE - 5))

if [ "$SCORE" -ge 90 ]; then
    GRADE="A"
    COLOR=$GREEN
elif [ "$SCORE" -ge 80 ]; then
    GRADE="B"
    COLOR=$YELLOW
elif [ "$SCORE" -ge 70 ]; then
    GRADE="C"
    COLOR=$YELLOW
else
    GRADE="D"
    COLOR=$RED
fi

echo -e "  Overall Quality Score: ${COLOR}$SCORE/100 (Grade: $GRADE)${NC}"

cat >> "$REPORT_FILE" << EOF

## Overall Quality Assessment

### Quality Score: $SCORE/100 (Grade: $GRADE)

### Recommendations
EOF

# Add recommendations based on findings
if [ "$BUILD_STATUS" = "❌ FAIL" ]; then
    echo "- 🔴 **CRITICAL**: Fix build errors immediately" >> "$REPORT_FILE"
fi

if [ "$TEST_STATUS" = "❌ FAIL" ]; then
    echo "- 🔴 **HIGH**: Fix failing tests" >> "$REPORT_FILE"
fi

if [ -n "$DOC_PERCENTAGE" ] && [ "$DOC_PERCENTAGE" -lt 50 ]; then
    echo "- 🟡 **MEDIUM**: Improve documentation coverage (target: >70%)" >> "$REPORT_FILE"
fi

if [ -n "$PANIC_USAGE" ] && [ "$PANIC_USAGE" -gt 3 ]; then
    echo "- 🟡 **MEDIUM**: Reduce panic usage, prefer error returns" >> "$REPORT_FILE"
fi

if [ -n "$LARGE_FILES" ]; then
    echo "- 🟡 **LOW**: Consider refactoring large files (>500 lines)" >> "$REPORT_FILE"
fi

echo "- ✅ **GOOD**: Continue following Go best practices" >> "$REPORT_FILE"

echo
echo -e "${GREEN}Quality check complete!${NC}"
echo -e "📊 Report saved to: ${BLUE}$REPORT_FILE${NC}"
echo -e "🎯 Quality Score: ${COLOR}$SCORE/100 (Grade: $GRADE)${NC}"

# Clean up
rm -f /tmp/test_output.txt
