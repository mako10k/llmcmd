# llmcmd v3.0.1 Release Notes

## 🐛 Critical Bug Fix Release

### Overview
Version 3.0.1 addresses a critical bug in the message history management that caused infinite loops during LLM tool execution. This was a high-priority fix that ensures stable operation of the llmcmd tool.

### 🔧 Bug Fixes

#### Fixed: Infinite Loop in Tool Execution (Critical)
- **Issue**: LLM would repeat the same commands indefinitely after the first API call
- **Root Cause**: Message history was being reset on iteration 2+, causing LLM memory loss
- **Impact**: Tool execution would never complete, consuming API quota unnecessarily
- **Fix**: Preserve conversation history across API calls, only update system message with quota info
- **Location**: `internal/app/app.go` lines 185-193
- **Validation**: Test execution now shows proper 2-step execution (spawn → read) instead of infinite loops

### 🧹 Code Quality & Maintenance

#### Project Structure Cleanup
- **Removed**: 13 outdated documentation files and unused builtin implementations
- **Added**: `.jscpd.json` configuration for code duplication detection
- **Added**: `package.json` with jscpd development dependency and scripts
- **Updated**: `.gitignore` with Node.js dependencies and reports directory

#### Documentation Updates
- **Updated**: README.md with v3.0.1 version badge and Code Quality section
- **Updated**: docs/CONFIGURATION.md with Quota System settings
- **Updated**: .github/copilot-instructions.md with Phase 6 completion status

### 🔍 Code Quality Tools

#### jscpd Integration
New code duplication detection capabilities:

```bash
# Install dependencies (run once)
npm install

# Run duplicate detection
npm run cpd               # Basic detection
npm run cpd:report        # Generate HTML + JSON reports
npm run cpd:verbose       # Verbose output with details
```

Configuration highlights:
- Minimum 5 lines or 50 tokens for detection
- Skip large files (>1000 lines) like `engine.go`
- Generate detailed HTML reports with source highlighting
- Export JSON data for CI/CD integration

### 📊 Impact Assessment

#### Before Fix
- **Behavior**: Infinite loop with repeated identical API calls
- **API Usage**: Excessive quota consumption
- **User Experience**: Tool never completes execution

#### After Fix
- **Behavior**: Clean 2-step execution (spawn → read → complete)
- **API Usage**: Minimal, appropriate quota usage
- **User Experience**: Fast, reliable tool completion

### 🚀 Installation

Download the latest release binary for your platform:

```bash
# Linux AMD64
wget https://github.com/mako10k/llmcmd/releases/download/v3.0.1/llmcmd-v3.0.1-linux-amd64.tar.gz
tar -xzf llmcmd-v3.0.1-linux-amd64.tar.gz
chmod +x llmcmd-v3.0.1-linux-amd64
sudo mv llmcmd-v3.0.1-linux-amd64 /usr/local/bin/llmcmd
```

### 🔄 Upgrade Recommendation

**Highly Recommended**: All users should upgrade to v3.0.1 immediately due to the critical nature of the infinite loop bug fix.

### 🧪 Testing

Validated scenarios:
- ✅ Basic file operations (read/write)
- ✅ Pipe command execution
- ✅ Multi-step tool chains
- ✅ Quota system functionality
- ✅ Configuration validation

### 🎯 Next Steps

- Monitor system stability with the fix
- Consider merging experimental shell tool features from feature branch in future releases
- Continue code quality improvements using jscpd reports

---

**Release Date**: 2025-07-30
**Version**: 3.0.1  
**Priority**: Critical Bug Fix
**Compatibility**: Backward compatible with v3.0.0 configurations
