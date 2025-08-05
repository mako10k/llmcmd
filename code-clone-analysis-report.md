# Code Clone Analysis Report

**Generated:** 2025-08-05 06:29
**Project:** llmcmd
**Branch:** feature/tool-layer-clean

## Summary

- **Total files analyzed:** 81 Go files
- **Total lines of code:** 19,794 lines
- **Code clones detected:** 36 exact clones
- **Duplicated lines:** 505 lines (2.57% of total)
- **Average duplication per file:** 14.03 lines

## Critical Issues (>30% duplication)

### 🔴 High Priority Refactoring Required

1. **internal/tools/builtin/head.go** - 43.1% (25 lines)
   - Similar to tail.go implementation
   - **Recommendation:** Extract common functionality into shared utility

2. **internal/tools/builtin/tail.go** - 36.76% (25 lines)
   - Duplicate of head.go patterns
   - **Recommendation:** Create shared line processing functions

## Medium Priority Issues (20-30% duplication)

3. **internal/llmsh/commands_extra.go** - 28.57% (90 lines)
   - High duplication with conversion commands
   - **Recommendation:** Refactor common command patterns

4. **internal/llmsh/commands/conversion.go** - 24.57% (86 lines)
   - Overlaps with commands_extra.go
   - **Recommendation:** Consolidate conversion logic

5. **internal/app/vfs_fsproxy_adapter_test.go** - 22.44% (46 lines)
   - Test code duplication
   - **Recommendation:** Extract test helper functions

## Lower Priority Issues (10-20% duplication)

6. **internal/llmsh/vfs_fsproxy_integration_test.go** - 18.98% (52 lines)
7. **internal/llmsh/parser/parser_test.go** - 17.54% (30 lines)
8. **internal/llmsh/commands/calculation.go** - 16.02% (54 lines)
9. **internal/llmsh/commands/encoding.go** - 14.59% (48 lines)
10. **internal/security/audit_test.go** - 14.06% (44 lines)
11. **internal/app/vfs_adapter_integration_test.go** - 13.61% (46 lines)
12. **internal/tools/engine.go** - 12.25% (146 lines)
13. **internal/openai/types.go** - 11.62% (48 lines)
14. **internal/openai/client.go** - 10.5% (98 lines)
15. **internal/app/fsproxy_commands_test.go** - 10.35% (50 lines)

## Common Clone Patterns Identified

### 1. **Command Processing Patterns**
- Location: `internal/llmsh/commands/` directory
- Pattern: Similar argument parsing and validation logic
- Solution: Create abstract command base class or interface

### 2. **Test Setup Patterns**
- Location: Various `*_test.go` files
- Pattern: Repeated test setup and teardown code
- Solution: Extract test helper functions and fixtures

### 3. **File I/O Operations**
- Location: `internal/tools/builtin/head.go` and `tail.go`
- Pattern: Similar file reading and line processing
- Solution: Create shared file processing utilities

### 4. **Error Handling Patterns**
- Location: Multiple files in `internal/openai/` and `internal/app/`
- Pattern: Repeated error checking and formatting
- Solution: Standardize error handling through helper functions

## Large Files Requiring Attention

Files exceeding 500 lines that should be considered for refactoring:

1. **internal/app/fsproxy.go** - 1,528 lines ⚠️ **Extremely large**
2. **internal/tools/engine.go** - 1,193 lines ⚠️ **Very large**
3. **internal/openai/client.go** - 934 lines
4. **internal/app/app.go** - 852 lines
5. **internal/cli/config.go** - 795 lines
6. **internal/app/fsproxy_test.go** - 639 lines
7. **internal/llmsh/vfs.go** - 566 lines
8. **internal/tools/builtin/help.go** - 564 lines
9. **internal/app/vfs.go** - 539 lines

## Recommendations

### Immediate Actions (Priority 1)

1. **Refactor head.go and tail.go**
   ```go
   // Create shared utility: internal/tools/builtin/lineprocessor.go
   package builtin
   
   func ProcessLines(reader io.Reader, processor func([]string) []string) error {
       // Common line processing logic
   }
   ```

2. **Extract command pattern utilities**
   ```go
   // Create: internal/llmsh/commands/base.go
   package commands
   
   type CommandBase struct {
       // Common command fields and methods
   }
   ```

### Medium-term Actions (Priority 2)

3. **Create test helper package**
   ```go
   // Create: internal/testutil/helpers.go
   package testutil
   
   func SetupTestEnvironment() (*TestContext, func()) {
       // Common test setup
   }
   ```

4. **Refactor large files**
   - Split `fsproxy.go` into multiple focused files
   - Break down `engine.go` by functionality
   - Separate `client.go` into request/response handlers

### Long-term Actions (Priority 3)

5. **Implement design patterns**
   - Strategy pattern for different command types
   - Factory pattern for object creation
   - Observer pattern for event handling

6. **Code organization improvements**
   - Group related functionality into packages
   - Establish clear interfaces between layers
   - Implement dependency injection

## Metrics Tracking

- **Before refactoring:** 2.57% code duplication
- **Target after refactoring:** <1.5% code duplication
- **Files needing immediate attention:** 5 files with >20% duplication
- **Large files to split:** 9 files with >500 lines

## Quality Score

**Current Code Quality Score: 75/100**

Deductions:
- Large files (9 files > 500 lines): -25 points
- High duplication files (5 files > 20%): -15 points

**Target Score: 90+/100**
