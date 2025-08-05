#!/bin/bash

echo "=== Testing diff command ==="

# Create test files
echo -e "line1\nline2\nline3" > /tmp/original.txt
echo -e "line1\nmodified_line2\nline3\nline4" > /tmp/modified.txt

# Test diff command
echo "Original file:"
cat /tmp/original.txt
echo -e "\nModified file:"
cat /tmp/modified.txt

echo -e "\n=== Testing diff via llmcmd ==="
echo "$(cat /tmp/original.txt)
---LLMCMD_DIFF_SEPARATOR---
$(cat /tmp/modified.txt)" | ./cmd/llmcmd/llmcmd diff > /tmp/test.diff

echo -e "\n=== Generated diff ==="
cat /tmp/test.diff

echo -e "\n=== Testing patch command ==="
# Test patch with the generated diff
echo "$(cat /tmp/original.txt)
---LLMCMD_PATCH_SEPARATOR---
$(cat /tmp/test.diff)" | ./cmd/llmcmd/llmcmd patch > /tmp/patched.txt

echo -e "\n=== Patched result ==="
cat /tmp/patched.txt

echo -e "\n=== Verification ==="
echo "Expected (modified.txt):"
cat /tmp/modified.txt
echo -e "\nActual (patched.txt):"
cat /tmp/patched.txt

if diff -q /tmp/modified.txt /tmp/patched.txt > /dev/null; then
    echo "✅ SUCCESS: Patch applied correctly!"
else
    echo "❌ FAILURE: Patch result doesn't match expected"
fi

# Cleanup
rm -f /tmp/original.txt /tmp/modified.txt /tmp/test.diff /tmp/patched.txt
