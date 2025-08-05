#!/bin/bash

echo "=== Testing diff with simple input ==="

# Very simple test case
echo -e "hello\n---LLMCMD_DIFF_SEPARATOR---\nworld" | timeout 30 ./cmd/llmcmd/llmcmd diff

echo -e "\n=== Testing diff with builtin function directly ==="

# Our previous test that works
go run cmd/test_diff_patch/main.go
