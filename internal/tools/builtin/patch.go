package builtin

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

// PatchChunk represents a chunk in a unified diff patch
type PatchChunk struct {
	OldStart, OldLines int
	NewStart, NewLines int
	Changes            []PatchLine
}

// PatchLine represents a line in a patch chunk
type PatchLine struct {
	Type    string // " " (context), "-" (delete), "+" (add)
	Content string
}

// Patch applies a unified diff patch to text input
// Input format: original_text + ---LLMCMD_PATCH_SEPARATOR--- + patch_content
// Args: [--validate] - optional pre-validation without applying patch
func Patch(args []string, stdin io.Reader, stdout io.Writer) error {
	if handled, _ := HandleHelp(args, stdout, `patch - Apply unified diff patches to text

Usage: patch [--dry-run]

Options:
  --dry-run         Don't actually apply patch (validation only)
  --help, -h        Show this help message

Input format: original_text + ---LLMCMD_PATCH_SEPARATOR--- + patch_content
`); handled {
		return nil
	}

	// Parse arguments
	dryRun := false
	for _, arg := range args {
		switch arg {
		case "--dry-run":
			dryRun = true
		case "--help", "-h":
			// Already handled above; ignore silently
		default:
			return fmt.Errorf("patch: unknown argument %q. Use --help for usage information", arg)
		}
	}

	content, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("patch: failed to read input: %w", err)
	}

	// Split input by separator
	parts := strings.Split(string(content), "---LLMCMD_PATCH_SEPARATOR---")
	if len(parts) != 2 {
		return fmt.Errorf("patch: input must contain exactly one ---LLMCMD_PATCH_SEPARATOR---")
	}

	originalText := strings.TrimSpace(parts[0])
	patchContent := strings.TrimSpace(parts[1])

	if dryRun {
		// Dry-run mode: only check if patch is valid
		err := validatePatch(originalText, patchContent)
		if err != nil {
			fmt.Fprintf(stdout, "DRY-RUN FAILED: %v\n", err)
			return nil // Don't return error for validation failure
		} else {
			fmt.Fprintf(stdout, "DRY-RUN SUCCESS: patch can be applied cleanly\n")
			return nil
		}
	}

	// Apply patch
	result, err := applyPatch(originalText, patchContent)
	if err != nil {
		return fmt.Errorf("patch: failed to apply patch: %w", err)
	}

	fmt.Fprint(stdout, result)
	return nil
}

// validatePatch checks if a patch can be applied without actually applying it
func validatePatch(originalText, patchContent string) error {
	lines := strings.Split(originalText, "\n")
	patchLines := strings.Split(patchContent, "\n")

	// Parse patch
	chunks, err := parsePatch(patchLines)
	if err != nil {
		return fmt.Errorf("patch parsing failed: %w", err)
	}

	// Validate each chunk without applying
	for i, chunk := range chunks {
		if err := validateChunk(lines, chunk); err != nil {
			return fmt.Errorf("chunk %d validation failed: %w", i+1, err)
		}
	}

	return nil
}

// validateChunk checks if a chunk can be applied without modifying the lines
func validateChunk(lines []string, chunk PatchChunk) error {
	// Convert to 0-based indexing
	startIndex := chunk.OldStart - 1
	if startIndex < 0 {
		return fmt.Errorf("invalid chunk start position %d: line numbers must be positive", chunk.OldStart)
	}
	if startIndex > len(lines) {
		return fmt.Errorf("chunk start position %d exceeds file length (%d lines)", chunk.OldStart, len(lines))
	}

	oldIndex := startIndex
	for _, change := range chunk.Changes {
		switch change.Type {
		case " ": // Context line
			if oldIndex >= len(lines) {
				return fmt.Errorf("context line %d beyond file end (%d lines)", oldIndex+1, len(lines))
			}
			if lines[oldIndex] != change.Content {
				return fmt.Errorf("context mismatch at line %d: expected %q, got %q",
					oldIndex+1, change.Content, lines[oldIndex])
			}
			oldIndex++
		case "-": // Delete line
			if oldIndex >= len(lines) {
				return fmt.Errorf("delete line %d beyond file end (%d lines)", oldIndex+1, len(lines))
			}
			if lines[oldIndex] != change.Content {
				return fmt.Errorf("delete mismatch at line %d: expected %q, got %q",
					oldIndex+1, change.Content, lines[oldIndex])
			}
			oldIndex++
		case "+": // Add line
			// Add operations don't need validation against existing content
		}
	}

	return nil
}

// applyPatch applies a unified diff patch to the original text
func applyPatch(originalText, patchContent string) (string, error) {
	lines := strings.Split(originalText, "\n")
	patchLines := strings.Split(patchContent, "\n")

	// Parse patch
	chunks, err := parsePatch(patchLines)
	if err != nil {
		return "", fmt.Errorf("patch parsing failed: %w", err)
	}

	// Apply chunks in reverse order to maintain line numbers
	for i := len(chunks) - 1; i >= 0; i-- {
		chunk := chunks[i]
		lines, err = applyChunk(lines, chunk)
		if err != nil {
			return "", fmt.Errorf("chunk %d application failed: %w", len(chunks)-i, err)
		}
	}

	return strings.Join(lines, "\n"), nil
}

// parsePatch parses unified diff format into patch chunks
func parsePatch(patchLines []string) ([]PatchChunk, error) {
	var chunks []PatchChunk
	var currentChunk *PatchChunk

	for i, line := range patchLines {
		if strings.HasPrefix(line, "@@") {
			// New chunk header
			if currentChunk != nil {
				chunks = append(chunks, *currentChunk)
			}

			// Parse @@ -oldStart,oldLines +newStart,newLines @@
			parts := strings.Fields(line)
			if len(parts) < 3 {
				return nil, fmt.Errorf("invalid chunk header at line %d: %q (expected format: @@ -start,count +start,count @@)", i+1, line)
			}

			oldPart := strings.TrimPrefix(parts[1], "-")
			newPart := strings.TrimPrefix(parts[2], "+")

			oldStart, oldLines, err := parseRange(oldPart)
			if err != nil {
				return nil, fmt.Errorf("invalid old range at line %d: %w", i+1, err)
			}

			newStart, newLines, err := parseRange(newPart)
			if err != nil {
				return nil, fmt.Errorf("invalid new range at line %d: %w", i+1, err)
			}

			currentChunk = &PatchChunk{
				OldStart: oldStart,
				OldLines: oldLines,
				NewStart: newStart,
				NewLines: newLines,
			}
		} else if currentChunk != nil && len(line) > 0 && (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "-") || strings.HasPrefix(line, "+")) {
			// Patch line
			currentChunk.Changes = append(currentChunk.Changes, PatchLine{
				Type:    line[:1],
				Content: line[1:],
			})
		}
	}

	if currentChunk != nil {
		chunks = append(chunks, *currentChunk)
	}

	if len(chunks) == 0 {
		return nil, fmt.Errorf("no valid patch chunks found (missing @@ headers or patch content)")
	}

	return chunks, nil
}

// parseRange parses a range like "1,3" or "1" and returns start and count
func parseRange(rangeStr string) (int, int, error) {
	if strings.Contains(rangeStr, ",") {
		parts := strings.Split(rangeStr, ",")
		if len(parts) != 2 {
			return 0, 0, fmt.Errorf("invalid range format %q (expected 'start,count')", rangeStr)
		}
		start, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0, 0, fmt.Errorf("invalid start number %q: %w", parts[0], err)
		}
		count, err := strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0, fmt.Errorf("invalid count number %q: %w", parts[1], err)
		}
		return start, count, nil
	} else {
		start, err := strconv.Atoi(rangeStr)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid line number %q: %w", rangeStr, err)
		}
		return start, 1, nil
	}
}

// applyChunk applies a single patch chunk to the lines
func applyChunk(lines []string, chunk PatchChunk) ([]string, error) {
	// Convert to 0-based indexing
	startIndex := chunk.OldStart - 1
	if startIndex < 0 {
		return nil, fmt.Errorf("invalid chunk start position %d: line numbers must be positive", chunk.OldStart)
	}
	if startIndex > len(lines) {
		return nil, fmt.Errorf("chunk start position %d exceeds file length (%d lines): patch may be for wrong file", chunk.OldStart, len(lines))
	}

	var result []string
	result = append(result, lines[:startIndex]...)

	// Apply changes
	oldIndex := startIndex
	for _, change := range chunk.Changes {
		switch change.Type {
		case " ": // Context line
			if oldIndex >= len(lines) {
				return nil, fmt.Errorf("context line %d beyond file end (%d lines): patch chunk @@ -%d,%d +%d,%d @@ exceeds file boundaries",
					oldIndex+1, len(lines), chunk.OldStart, chunk.OldLines, chunk.NewStart, chunk.NewLines)
			}
			if lines[oldIndex] != change.Content {
				return nil, fmt.Errorf("context mismatch at line %d: patch expects %q but file contains %q\n"+
					"  Hint: patch may be outdated or for different version of file",
					oldIndex+1, change.Content, lines[oldIndex])
			}
			result = append(result, change.Content)
			oldIndex++
		case "-": // Delete line
			if oldIndex >= len(lines) {
				return nil, fmt.Errorf("delete line %d beyond file end (%d lines): cannot delete non-existent line",
					oldIndex+1, len(lines))
			}
			if lines[oldIndex] != change.Content {
				return nil, fmt.Errorf("delete mismatch at line %d: patch wants to delete %q but file contains %q\n"+
					"  Hint: file may have been modified since patch was created",
					oldIndex+1, change.Content, lines[oldIndex])
			}
			// Skip this line (delete it)
			oldIndex++
		case "+": // Add line
			result = append(result, change.Content)
			// Don't increment oldIndex for additions
		}
	}

	// Add remaining lines after the chunk
	result = append(result, lines[oldIndex:]...)

	return result, nil
}
