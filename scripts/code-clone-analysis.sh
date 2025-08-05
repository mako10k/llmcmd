#!/bin/bash

# =============================================================================
# Code Clone Analysis Dashboard for llmcmd Project
# =============================================================================

set -euo pipefail

# Color definitions
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPORT_DIR="$PROJECT_ROOT/code-analysis-reports"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")

# Create report directory
mkdir -p "$REPORT_DIR"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  Code Clone Analysis Dashboard${NC}"
echo -e "${BLUE}  Project: llmcmd${NC}"
echo -e "${BLUE}  Date: $(date)${NC}"
echo -e "${BLUE}========================================${NC}"
echo

# =============================================================================
# Function: Print section header
# =============================================================================
print_section() {
    echo -e "\n${CYAN}=== $1 ===${NC}"
}

# =============================================================================
# Function: Check if command exists
# =============================================================================
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# =============================================================================
# 1. Project Statistics
# =============================================================================
print_section "Project Overview"

echo -e "${YELLOW}Repository Information:${NC}"
echo "  Current branch: $(git branch --show-current)"
echo "  Total commits: $(git rev-list --count HEAD)"
echo "  Contributors: $(git log --format='%an' | sort -u | wc -l)"

echo -e "\n${YELLOW}Code Metrics:${NC}"
total_go_files=$(find internal/ cmd/ -name "*.go" | wc -l)
total_test_files=$(find internal/ cmd/ -name "*_test.go" | wc -l)
total_lines=$(find internal/ cmd/ -name "*.go" | xargs wc -l | tail -1 | awk '{print $1}')
non_test_files=$((total_go_files - total_test_files))

echo "  Total Go files: $total_go_files"
echo "  Production files: $non_test_files"
echo "  Test files: $total_test_files"
echo "  Total lines of code: $total_lines"

# =============================================================================
# 2. File Size Analysis
# =============================================================================
print_section "Large Files Analysis"

echo -e "${YELLOW}Top 10 Largest Files (by lines):${NC}"
find internal/ cmd/ -name "*.go" | xargs wc -l | sort -n | tail -11 | head -10 | while read -r lines file; do
    if [ "$lines" -gt 500 ]; then
        echo -e "  ${RED}$lines${NC} $file ${RED}(>500 lines - consider refactoring)${NC}"
    elif [ "$lines" -gt 300 ]; then
        echo -e "  ${YELLOW}$lines${NC} $file ${YELLOW}(>300 lines - monitor)${NC}"
    else
        echo -e "  ${GREEN}$lines${NC} $file"
    fi
done

# =============================================================================
# 3. JSCPD Code Clone Analysis
# =============================================================================
print_section "Code Clone Detection"

if command_exists npx; then
    echo -e "${YELLOW}Running JSCPD analysis...${NC}"
    
    # Create jscpd report directory
    JSCPD_REPORT_DIR="$REPORT_DIR/jscpd-$TIMESTAMP"
    mkdir -p "$JSCPD_REPORT_DIR"
    
    # Run JSCPD with comprehensive settings
    npx jscpd \
        --reporters html,json,badge \
        --output "$JSCPD_REPORT_DIR" \
        internal/ cmd/ \
        --min-lines 5 \
        --min-tokens 50 \
        --ignore "**/mock/**,**/*_mock.go,**/vendor/**" \
        --threshold 10 \
        --silent || true
    
    # Parse JSON report for summary
    if [ -f "$JSCPD_REPORT_DIR/jscpd-report.json" ]; then
        echo -e "\n${YELLOW}Clone Detection Results:${NC}"
        
        # Count total clones
        total_clones=$(grep -o '"format":"go"' "$JSCPD_REPORT_DIR/jscpd-report.json" | wc -l)
        echo "  Total code clones detected: $total_clones"
        
        # Extract highest duplication percentages
        echo -e "\n${YELLOW}Files with High Duplication:${NC}"
        if command_exists jq; then
            jq -r '.statistics.formats.go.sources | to_entries[] | select(.value.percentage > 10) | "  \(.key): \(.value.percentage)% (\(.value.duplicatedLines) lines)"' "$JSCPD_REPORT_DIR/jscpd-report.json" | sort -k2 -nr | head -10
        else
            echo "  (jq not available - check HTML report for details)"
        fi
        
        echo -e "\n${GREEN}HTML Report: $JSCPD_REPORT_DIR/html/index.html${NC}"
        echo -e "${GREEN}JSON Report: $JSCPD_REPORT_DIR/jscpd-report.json${NC}"
    fi
else
    echo -e "${RED}npx not available - skipping JSCPD analysis${NC}"
    echo "  To install: npm install -g jscpd"
fi

# =============================================================================
# 4. Go Vet Analysis
# =============================================================================
print_section "Go Vet Analysis"

echo -e "${YELLOW}Running go vet...${NC}"
if go vet ./...; then
    echo -e "${GREEN}✓ No issues found by go vet${NC}"
else
    echo -e "${RED}✗ Issues found by go vet (see output above)${NC}"
fi

# =============================================================================
# 5. Gofmt Analysis
# =============================================================================
print_section "Code Formatting Analysis"

echo -e "${YELLOW}Checking gofmt compliance...${NC}"
unformatted_files=$(gofmt -l internal/ cmd/ 2>/dev/null || true)
if [ -z "$unformatted_files" ]; then
    echo -e "${GREEN}✓ All files are properly formatted${NC}"
else
    echo -e "${RED}✗ Files needing formatting:${NC}"
    echo "$unformatted_files" | sed 's/^/  /'
fi

# =============================================================================
# 6. Function Complexity Analysis (Simple)
# =============================================================================
print_section "Function Complexity Analysis"

echo -e "${YELLOW}Analyzing function complexity...${NC}"
echo "  Functions with high cyclomatic complexity patterns:"

# Look for deeply nested functions (simple heuristic)
grep -rn "^[[:space:]]*if.*{[[:space:]]*$" internal/ cmd/ | wc -l > /tmp/if_count
grep -rn "^[[:space:]]*for.*{[[:space:]]*$" internal/ cmd/ | wc -l > /tmp/for_count
grep -rn "^[[:space:]]*switch.*{[[:space:]]*$" internal/ cmd/ | wc -l > /tmp/switch_count

if_count=$(cat /tmp/if_count)
for_count=$(cat /tmp/for_count)
switch_count=$(cat /tmp/switch_count)

echo "  - If statements: $if_count"
echo "  - For loops: $for_count"
echo "  - Switch statements: $switch_count"

# Find functions with many if/for/switch statements
echo -e "\n${YELLOW}Files with potentially high complexity:${NC}"
find internal/ cmd/ -name "*.go" -exec grep -l "if.*{.*if.*{" {} \; | head -5 | while read -r file; do
    nested_count=$(grep -c "if.*{.*if.*{" "$file" 2>/dev/null || echo 0)
    if [ "$nested_count" -gt 0 ]; then
        echo "  $file: $nested_count nested patterns"
    fi
done

rm -f /tmp/if_count /tmp/for_count /tmp/switch_count

# =============================================================================
# 7. Test Coverage Analysis
# =============================================================================
print_section "Test Coverage Analysis"

echo -e "${YELLOW}Running test coverage analysis...${NC}"
COVERAGE_FILE="$REPORT_DIR/coverage-$TIMESTAMP.out"

if go test -coverprofile="$COVERAGE_FILE" ./... >/dev/null 2>&1; then
    coverage_percent=$(go tool cover -func="$COVERAGE_FILE" | tail -1 | awk '{print $3}')
    echo "  Overall test coverage: $coverage_percent"
    
    # Generate HTML coverage report
    COVERAGE_HTML="$REPORT_DIR/coverage-$TIMESTAMP.html"
    go tool cover -html="$COVERAGE_FILE" -o "$COVERAGE_HTML"
    echo -e "${GREEN}Coverage report: $COVERAGE_HTML${NC}"
    
    # Show packages with low coverage
    echo -e "\n${YELLOW}Packages with low coverage (<50%):${NC}"
    go tool cover -func="$COVERAGE_FILE" | awk '$3 != "total:" && $3 < "50.0%" {print "  " $1 ": " $3}' | head -10
else
    echo -e "${RED}✗ Test coverage analysis failed${NC}"
fi

# =============================================================================
# 8. Dependency Analysis
# =============================================================================
print_section "Dependency Analysis"

echo -e "${YELLOW}Module dependencies:${NC}"
go list -m all | wc -l | xargs echo "  Total dependencies:"

echo -e "\n${YELLOW}Direct dependencies:${NC}"
go list -m -f '{{if not .Indirect}}{{.Path}}{{end}}' all | grep -v "^$" | wc -l | xargs echo "  Direct dependencies:"

echo -e "\n${YELLOW}Vulnerability check:${NC}"
if command_exists govulncheck; then
    govulncheck ./... | grep -E "(vulnerability|No vulnerabilities)" || echo "  No vulnerabilities found"
else
    echo "  govulncheck not available (install with: go install golang.org/x/vuln/cmd/govulncheck@latest)"
fi

# =============================================================================
# 9. Git Repository Health
# =============================================================================
print_section "Repository Health"

echo -e "${YELLOW}Recent commit activity:${NC}"
echo "  Commits in last 30 days: $(git log --since="30 days ago" --oneline | wc -l)"
echo "  Average commits per day: $(git log --since="30 days ago" --oneline | wc -l | awk '{printf "%.1f", $1/30}')"

echo -e "\n${YELLOW}Branch information:${NC}"
echo "  Total branches: $(git branch -a | wc -l)"
echo "  Remote branches: $(git branch -r | wc -l)"

echo -e "\n${YELLOW}Repository size:${NC}"
repo_size=$(du -sh .git 2>/dev/null | cut -f1 || echo "unknown")
working_size=$(du -sh --exclude=.git . 2>/dev/null | cut -f1 || echo "unknown")
echo "  .git directory: $repo_size"
echo "  Working directory: $working_size"

# =============================================================================
# 10. Summary and Recommendations
# =============================================================================
print_section "Summary and Recommendations"

echo -e "${YELLOW}Code Quality Assessment:${NC}"

# Calculate quality score based on various factors
quality_score=100

# Deduct points for large files
large_files=$(find internal/ cmd/ -name "*.go" | xargs wc -l | awk '$1 > 500 {print $2}' | wc -l)
quality_score=$((quality_score - large_files * 5))

# Deduct points for unformatted files
unformatted_count=$(gofmt -l internal/ cmd/ 2>/dev/null | wc -l)
quality_score=$((quality_score - unformatted_count * 10))

# Ensure score doesn't go below 0
[ $quality_score -lt 0 ] && quality_score=0

echo "  Estimated code quality score: $quality_score/100"

echo -e "\n${YELLOW}Recommendations:${NC}"

if [ $large_files -gt 0 ]; then
    echo -e "  ${RED}•${NC} Consider refactoring files with >500 lines"
fi

if [ $unformatted_count -gt 0 ]; then
    echo -e "  ${RED}•${NC} Run 'gofmt -w .' to fix formatting issues"
fi

if command_exists npx && [ -f "$JSCPD_REPORT_DIR/jscpd-report.json" ]; then
    high_dup_count=$(jq -r '.statistics.formats.go.sources | to_entries[] | select(.value.percentage > 20)' "$JSCPD_REPORT_DIR/jscpd-report.json" 2>/dev/null | wc -l)
    if [ "$high_dup_count" -gt 0 ]; then
        echo -e "  ${RED}•${NC} Address code clones with >20% duplication"
    fi
fi

echo -e "  ${GREEN}•${NC} Continue regular code reviews and refactoring"
echo -e "  ${GREEN}•${NC} Maintain test coverage above 80%"
echo -e "  ${GREEN}•${NC} Monitor cyclomatic complexity in large functions"

# =============================================================================
# 11. Report Generation
# =============================================================================
print_section "Report Generation"

SUMMARY_REPORT="$REPORT_DIR/summary-$TIMESTAMP.md"

cat > "$SUMMARY_REPORT" << EOF
# Code Analysis Summary Report

**Generated:** $(date)
**Project:** llmcmd
**Branch:** $(git branch --show-current)

## Metrics
- Total Go files: $total_go_files
- Total lines: $total_lines
- Quality score: $quality_score/100

## Files Requiring Attention
$(find internal/ cmd/ -name "*.go" | xargs wc -l | awk '$1 > 500 {print "- " $2 " (" $1 " lines)"}')

## Recommendations
$([ $large_files -gt 0 ] && echo "- Refactor large files (>500 lines)")
$([ $unformatted_count -gt 0 ] && echo "- Fix formatting issues")
- Continue regular code quality monitoring

## Reports Generated
- Summary: $SUMMARY_REPORT
$([ -f "$JSCPD_REPORT_DIR/html/index.html" ] && echo "- Code clones: $JSCPD_REPORT_DIR/html/index.html")
$([ -f "$COVERAGE_HTML" ] && echo "- Test coverage: $COVERAGE_HTML")
EOF

echo -e "${GREEN}Summary report saved: $SUMMARY_REPORT${NC}"

# =============================================================================
# 12. Final Status
# =============================================================================
echo
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  Analysis Complete${NC}"
echo -e "${BLUE}  Reports saved to: $REPORT_DIR${NC}"
echo -e "${BLUE}========================================${NC}"

exit 0
