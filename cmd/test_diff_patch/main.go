package main

import (
	"fmt"
	"strings"
	"os"
	"github.com/mako10k/llmcmd/internal/tools/builtin"
)

func main() {
	fmt.Println("=== Testing diff function directly ===")
	
	input := `line1
line2
line3
---LLMCMD_DIFF_SEPARATOR---
line1
modified_line2
line3
line4`

	reader := strings.NewReader(input)
	var output strings.Builder
	
	err := builtin.Diff([]string{}, reader, &output)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Println("Generated diff:")
	fmt.Println(output.String())
	
	fmt.Println("\n=== Testing patch function directly ===")
	
	// Test patch with the generated diff
	patchInput := fmt.Sprintf(`line1
line2
line3
---LLMCMD_PATCH_SEPARATOR---
%s`, output.String())

	patchReader := strings.NewReader(patchInput)
	var patchOutput strings.Builder
	
	err = builtin.Patch([]string{}, patchReader, &patchOutput)
	if err != nil {
		fmt.Printf("Patch error: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Println("Patched result:")
	fmt.Println(patchOutput.String())
	
	expected := `line1
modified_line2
line3
line4`
	
	if strings.TrimSpace(patchOutput.String()) == strings.TrimSpace(expected) {
		fmt.Println("✅ SUCCESS: Patch applied correctly!")
	} else {
		fmt.Println("❌ FAILURE: Patch result doesn't match expected")
		fmt.Printf("Expected:\n%s\n", expected)
		fmt.Printf("Got:\n%s\n", patchOutput.String())
	}
}
