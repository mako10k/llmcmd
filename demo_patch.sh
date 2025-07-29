#!/bin/bash

echo "=== Patch Application Demo Using llmcmd spawn tool pattern ==="
echo "This demonstrates the spawn → write → read pattern for patch operations"
echo

# Check if files exist
SOURCE_FILE="test_data/example_source.go"
if [ ! -f "$SOURCE_FILE" ]; then
    echo "Error: Source file $SOURCE_FILE not found"
    exit 1
fi

echo "1. Original file information:"
echo "   File: $SOURCE_FILE"
echo "   Size: $(wc -c < $SOURCE_FILE) bytes"
echo "   Lines: $(wc -l < $SOURCE_FILE) lines"
echo

echo "2. Creating a patch by modifying the source:"
# Create a modified version
cp "$SOURCE_FILE" "${SOURCE_FILE}.modified"

# Add a simple modification (add a comment at the top)
sed -i '1i// Modified version with additional features' "${SOURCE_FILE}.modified"

# Add another line 
sed -i '/type User struct/a\	// Additional user tracking fields' "${SOURCE_FILE}.modified"

echo "   Modified file: ${SOURCE_FILE}.modified"
echo "   New size: $(wc -c < ${SOURCE_FILE}.modified) bytes"
echo "   New lines: $(wc -l < ${SOURCE_FILE}.modified) lines"
echo

echo "3. Generating patch file:"
diff -u "$SOURCE_FILE" "${SOURCE_FILE}.modified" > "${SOURCE_FILE}.patch" || true
echo "   Patch file: ${SOURCE_FILE}.patch"
echo "   Patch size: $(wc -c < ${SOURCE_FILE}.patch) bytes"
echo

echo "4. Patch content preview:"
head -20 "${SOURCE_FILE}.patch"
echo "   ..."
echo

echo "5. Applying patch using standard patch command:"
cp "$SOURCE_FILE" "${SOURCE_FILE}.test"
patch "${SOURCE_FILE}.test" < "${SOURCE_FILE}.patch"
echo "   Applied patch to: ${SOURCE_FILE}.test"
echo

echo "6. Verifying patch application:"
echo "   Original size: $(wc -c < $SOURCE_FILE) bytes"
echo "   Patched size: $(wc -c < ${SOURCE_FILE}.test) bytes"
echo "   Size difference: $(($(wc -c < ${SOURCE_FILE}.test) - $(wc -c < $SOURCE_FILE))) bytes"
echo

echo "7. Final diff to confirm changes:"
diff -u "$SOURCE_FILE" "${SOURCE_FILE}.test" || true
echo

echo "=== spawn tool pattern equivalent ==="
echo "For LLM usage, this would be:"
echo "1. spawn({cmd:'patch', args:['-p0', 'target_file']}) → {in_fd: N, out_fd: M}"
echo "2. write(N, patch_content, {eof: true})"
echo "3. read(M) → get patch application results"
echo
echo "Patch application demo complete!"
echo "Files created:"
echo "  - ${SOURCE_FILE}.modified (modified source)"
echo "  - ${SOURCE_FILE}.patch (patch file)"
echo "  - ${SOURCE_FILE}.test (patched result)"
