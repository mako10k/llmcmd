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
func Patch(args []string, stdin io.Reader, stdout io.Writer) error {
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

	// Apply patch
	result, err := applyPatch(originalText, patchContent)
	if err != nil {
		return fmt.Errorf("patch: failed to apply patch: %w", err)
	}

	fmt.Fprint(stdout, result)
	return nil
}

// applyPatch applies a unified diff patch to the original text
func applyPatch(originalText, patchContent string) (string, error) {
	lines := strings.Split(originalText, "\n")
	patchLines := strings.Split(patchContent, "\n")

	// Parse patch
	chunks, err := parsePatch(patchLines)
	if err != nil {
		return "", err
	}

	// Apply chunks in reverse order to maintain line numbers
	for i := len(chunks) - 1; i >= 0; i-- {
		chunk := chunks[i]
		lines, err = applyChunk(lines, chunk)
		if err != nil {
			return "", err
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
				return nil, fmt.Errorf("invalid chunk header at line %d: %s", i+1, line)
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
		} else if currentChunk != nil && (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "-") || strings.HasPrefix(line, "+")) {
			// Patch line
			if len(line) == 0 {
				continue
			}
			currentChunk.Changes = append(currentChunk.Changes, PatchLine{
				Type:    line[:1],
				Content: line[1:],
			})
		}
	}

	if currentChunk != nil {
		chunks = append(chunks, *currentChunk)
	}

	return chunks, nil
}

// parseRange parses a range like "1,3" or "1" and returns start and count
func parseRange(rangeStr string) (int, int, error) {
	if strings.Contains(rangeStr, ",") {
		parts := strings.Split(rangeStr, ",")
		start, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0, 0, err
		}
		count, err := strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0, err
		}
		return start, count, nil
	} else {
		start, err := strconv.Atoi(rangeStr)
		if err != nil {
			return 0, 0, err
		}
		return start, 1, nil
	}
}

// applyChunk applies a single patch chunk to the lines
func applyChunk(lines []string, chunk PatchChunk) ([]string, error) {
	// Convert to 0-based indexing
	startIndex := chunk.OldStart - 1
	if startIndex < 0 || startIndex > len(lines) {
		return nil, fmt.Errorf("invalid chunk start position: %d", chunk.OldStart)
	}

	var result []string
	result = append(result, lines[:startIndex]...)

	// Apply changes
	oldIndex := startIndex
	for _, change := range chunk.Changes {
		switch change.Type {
		case " ": // Context line
			if oldIndex >= len(lines) {
				return nil, fmt.Errorf("context line beyond file end")
			}
			if lines[oldIndex] != change.Content {
				return nil, fmt.Errorf("context mismatch at line %d: expected %q, got %q",
					oldIndex+1, change.Content, lines[oldIndex])
			}
			result = append(result, change.Content)
			oldIndex++
		case "-": // Delete line
			if oldIndex >= len(lines) {
				return nil, fmt.Errorf("delete line beyond file end")
			}
			if lines[oldIndex] != change.Content {
				return nil, fmt.Errorf("delete mismatch at line %d: expected %q, got %q",
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
