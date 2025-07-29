package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/mako10k/llmcmd/internal/tools/builtin"
)

func main() {
	fmt.Println("=== Testing Separated diff.go and patch.go ===")

	// Test 1: Diff functionality
	fmt.Println("\n1. Testing diff command...")

	file1 := "line 1\ncommon line\nline 3 original\nline 4"
	file2 := "line 1\ncommon line\nline 3 modified\nline 4\nline 5 added"

	diffInput := file1 + "\n---LLMCMD_DIFF_SEPARATOR---\n" + file2
	diffReader := strings.NewReader(diffInput)

	fmt.Println("Diff input:")
	fmt.Printf("File1: %q\n", file1)
	fmt.Printf("File2: %q\n", file2)

	fmt.Println("\nRunning diff command...")
	err := builtin.Diff([]string{}, diffReader, os.Stdout)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	// Test 2: Patch functionality
	fmt.Println("\n\n2. Testing patch command...")

	original := "line 1\ncommon line\nline 3 original\nline 4"
	patch := `--- a/file
+++ b/file
@@ -1,4 +1,5 @@
 line 1
 common line
-line 3 original
+line 3 modified
 line 4
+line 5 added`

	patchInput := original + "\n---LLMCMD_PATCH_SEPARATOR---\n" + patch
	patchReader := strings.NewReader(patchInput)

	fmt.Println("Patch input:")
	fmt.Printf("Original: %q\n", original)
	fmt.Printf("Patch:\n%s\n", patch)

	fmt.Println("\nRunning patch command...")
	err = builtin.Patch([]string{}, patchReader, os.Stdout)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	fmt.Println("\n\n=== Test Complete ===")
}
