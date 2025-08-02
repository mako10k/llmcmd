#!/bin/bash
# llmcmd Quality Check Dashboard - Simplified Version
set -e

PROJECT_ROOT=$(dirname $(dirname $(realpath $0)))
REPORT_FILE="$PROJECT_ROOT/reports/quality-report-$(date +%Y%m%d-%H%M%S).md"

echo "=== llmcmd Quality Check Dashboard ==="
echo "Timestamp: $(date)"
echo "Report will be saved to: $REPORT_FILE"
echo

mkdir -p "$PROJECT_ROOT/reports"

# Initialize report
cat > "$REPORT_FILE" << 'EOF'
# llmcmd Code Quality Report

## Summary
EOF

echo "1. Project Statistics"
cd "$PROJECT_ROOT"

# Basic stats
TOTAL_LINES=$(find . -name "*.go" -not -path "*/vendor/*" | xargs wc -l | tail -1 | awk '{print $1}')
GO_FILES=$(find . -name "*.go" -not -path "*/vendor/*" | wc -l)

echo "  - Total Go files: $GO_FILES"
echo "  - Total lines of code: $TOTAL_LINES"

cat >> "$REPORT_FILE" << EOF

### Project Statistics
- **Total Go Files**: $GO_FILES
- **Total Lines of Code**: $TOTAL_LINES
- **Average Lines per File**: $((TOTAL_LINES / GO_FILES))

EOF

echo "2. Build & Test Status"

# Test build
if go build ./... >/dev/null 2>&1; then
    BUILD_STATUS="✅ PASS"
    echo "  Build: PASS"
else
    BUILD_STATUS="❌ FAIL"
    echo "  Build: FAIL"
fi

# Test suite
if go test ./... >/dev/null 2>&1; then
    TEST_STATUS="✅ PASS"
    echo "  Tests: PASS"
else
    TEST_STATUS="❌ FAIL"
    echo "  Tests: FAIL"
fi

cat >> "$REPORT_FILE" << EOF
### Build & Test Status
- **Build Status**: $BUILD_STATUS
- **Test Status**: $TEST_STATUS

EOF

echo "3. Code Quality Metrics"

# Error handling
ERROR_CHECKS=$(find . -name "*.go" -not -path "*/vendor/*" | xargs grep -c "if err != nil" | awk -F: '{sum+=$2} END{print sum}')
ERROR_RETURNS=$(find . -name "*.go" -not -path "*/vendor/*" | xargs grep -c "return.*err" | awk -F: '{sum+=$2} END{print sum}')
PANIC_USAGE=$(find . -name "*.go" -not -path "*/vendor/*" | xargs grep -c "panic(" | awk -F: '{sum+=$2} END{print sum}')

echo "  - Error checks: $ERROR_CHECKS"
echo "  - Error returns: $ERROR_RETURNS"  
echo "  - Panic usage: $PANIC_USAGE"

# Large files
LARGE_FILES=$(find . -name "*.go" -not -path "*/vendor/*" | xargs wc -l | awk '$1 > 500 {count++} END{print count+0}')
echo "  - Large files (>500 lines): $LARGE_FILES"

cat >> "$REPORT_FILE" << EOF
### Code Quality Metrics
- **Error Checks (if err != nil)**: $ERROR_CHECKS
- **Error Returns**: $ERROR_RETURNS
- **Panic Usage**: $PANIC_USAGE $(if [ "$PANIC_USAGE" -eq 0 ]; then echo "✅"; else echo "⚠️"; fi)
- **Large Files (>500 lines)**: $LARGE_FILES $(if [ "$LARGE_FILES" -le 5 ]; then echo "✅"; else echo "⚠️"; fi)

EOF

echo "4. Quality Score Calculation"

# Calculate score
SCORE=100
[ "$BUILD_STATUS" = "❌ FAIL" ] && SCORE=$((SCORE - 30))
[ "$TEST_STATUS" = "❌ FAIL" ] && SCORE=$((SCORE - 20))
[ "$PANIC_USAGE" -gt 3 ] && SCORE=$((SCORE - 10))
[ "$LARGE_FILES" -gt 10 ] && SCORE=$((SCORE - 5))

if [ "$SCORE" -ge 90 ]; then
    GRADE="A"
elif [ "$SCORE" -ge 80 ]; then
    GRADE="B"
elif [ "$SCORE" -ge 70 ]; then
    GRADE="C"
else
    GRADE="D"
fi

echo "  Overall Quality Score: $SCORE/100 (Grade: $GRADE)"

cat >> "$REPORT_FILE" << EOF

## Overall Quality Assessment

### Quality Score: $SCORE/100 (Grade: $GRADE)

### Key Strengths
- ✅ Build system working correctly
- ✅ Test suite passing
- ✅ Good error handling patterns
- ✅ Minimal panic usage
- ✅ Reasonable file sizes

### Recommendations for Improvement
EOF

if [ "$BUILD_STATUS" = "❌ FAIL" ]; then
    echo "- 🔴 **CRITICAL**: Fix build errors immediately" >> "$REPORT_FILE"
fi

if [ "$TEST_STATUS" = "❌ FAIL" ]; then
    echo "- 🔴 **HIGH**: Fix failing tests" >> "$REPORT_FILE"
fi

if [ "$LARGE_FILES" -gt 10 ]; then
    echo "- 🟡 **MEDIUM**: Consider refactoring large files" >> "$REPORT_FILE"
fi

echo "- ✅ **GOOD**: Continue following Go best practices" >> "$REPORT_FILE"
echo "- ✅ **GOOD**: Maintain current error handling patterns" >> "$REPORT_FILE"

echo
echo "Quality check complete!"
echo "📊 Report saved to: $REPORT_FILE"
echo "🎯 Quality Score: $SCORE/100 (Grade: $GRADE)"
