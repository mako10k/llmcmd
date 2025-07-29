# 🚀 llmcmd v2.0.0 - Major Update

## ✨ New Features
- **4-Pattern Pipe Tool System**: Flexible background and foreground execution modes
- **Advanced Dependency Management**: Deadlock prevention with 1:1 and 1:many FD relationships  
- **Enhanced tee Tool**: Support for 1:many output distribution
- **Improved File Analysis**: Better handling of binary vs text files
- **Cross-platform Builds**: Native binaries for Linux, macOS, and Windows (AMD64/ARM64)

## 🔧 Tool Enhancements
- **pipe**: 4 execution patterns for maximum flexibility
- **tee**: Multiple output support with dependency tracking  
- **close**: Safe FD closure with dependency warnings
- **exit**: Graceful program termination

## 📦 Installation
```bash
# Quick install (Linux/macOS)
curl -sSL https://raw.githubusercontent.com/mako10k/llmcmd/main/install.sh | bash

# Manual installation
chmod +x llmcmd-*
sudo mv llmcmd-* /usr/local/bin/llmcmd
```

## 🎯 Usage
```bash
export OPENAI_API_KEY="your_api_key"
llmcmd "your task description"
echo "data" | llmcmd "process this"
```

This release represents a major advancement in LLM-powered command execution with sophisticated pipeline management and cross-platform support.
