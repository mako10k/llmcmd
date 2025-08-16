package builtin

import (
	"fmt"
	"io"
	"strings"
)

// DiffChunk represents a chunk of differences in a unified diff
type DiffChunk struct {
	OldStart, OldLines int
	NewStart, NewLines int
	Lines              []string
}

// Diff compares two text inputs separated by ---LLMCMD_DIFF_SEPARATOR---
// and outputs unified diff format
func Diff(args []string, stdin io.Reader, stdout io.Writer) error {
	if handled, _, err := HandleHelp(args, stdout, `diff - Compare two text inputs

Usage: diff [file...]

Input format: text1 + ---LLMCMD_DIFF_SEPARATOR--- + text2

Description:
	Compare two text sections and output unified diff format

Options:
	--help, -h        Show this help message

Examples:
	Input with separator to compare two texts
	Output shows unified diff format with +/- indicators
`); handled {
		return err
	}

	content, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("diff: failed to read input: %w", err)
	}

	// Split input by separator
	parts := strings.Split(string(content), "---LLMCMD_DIFF_SEPARATOR---")
	if len(parts) != 2 {
		return fmt.Errorf("diff: input must contain exactly one ---LLMCMD_DIFF_SEPARATOR---")
	}

	file1Lines := strings.Split(strings.TrimSpace(parts[0]), "\n")
	file2Lines := strings.Split(strings.TrimSpace(parts[1]), "\n")

	// Generate unified diff
	chunks := computeDiff(file1Lines, file2Lines)

	// Output unified diff format
	fmt.Fprintf(stdout, "--- a/file\n")
	fmt.Fprintf(stdout, "+++ b/file\n")

	for _, chunk := range chunks {
		fmt.Fprintf(stdout, "@@ -%d,%d +%d,%d @@\n",
			chunk.OldStart, chunk.OldLines,
			chunk.NewStart, chunk.NewLines)

		for _, line := range chunk.Lines {
			fmt.Fprintln(stdout, line)
		}
	}

	return nil
}

// computeDiff computes the differences between two slices of lines
func computeDiff(oldLines, newLines []string) []DiffChunk {
	var chunks []DiffChunk

	// Simple line-by-line comparison for now
	// This is a basic implementation - a real diff would use LCS algorithm

	oldIndex, newIndex := 0, 0
	contextLines := 3 // Number of context lines to show

	for oldIndex < len(oldLines) || newIndex < len(newLines) {
		chunk := DiffChunk{
			OldStart: oldIndex + 1,
			NewStart: newIndex + 1,
		}

		// Find differences
		chunkLines := []string{}
		chunkOldCount := 0
		chunkNewCount := 0

		// Add context before changes
		contextStart := max(0, oldIndex-contextLines)
		for i := contextStart; i < oldIndex && i < len(oldLines); i++ {
			chunkLines = append(chunkLines, " "+oldLines[i])
			chunkOldCount++
			chunkNewCount++
		}

		// Process differences
		for oldIndex < len(oldLines) && newIndex < len(newLines) {
			if oldLines[oldIndex] == newLines[newIndex] {
				// Lines match, add as context
				chunkLines = append(chunkLines, " "+oldLines[oldIndex])
				chunkOldCount++
				chunkNewCount++
				oldIndex++
				newIndex++
				break
			} else {
				// Lines differ
				chunkLines = append(chunkLines, "-"+oldLines[oldIndex])
				chunkLines = append(chunkLines, "+"+newLines[newIndex])
				chunkOldCount++
				chunkNewCount++
				oldIndex++
				newIndex++
			}
		}

		// Handle remaining lines in old file
		for oldIndex < len(oldLines) {
			chunkLines = append(chunkLines, "-"+oldLines[oldIndex])
			chunkOldCount++
			oldIndex++
		}

		// Handle remaining lines in new file
		for newIndex < len(newLines) {
			chunkLines = append(chunkLines, "+"+newLines[newIndex])
			chunkNewCount++
			newIndex++
		}

		if len(chunkLines) > 0 {
			chunk.Lines = chunkLines
			chunk.OldLines = chunkOldCount
			chunk.NewLines = chunkNewCount
			chunks = append(chunks, chunk)
		}
	}

	return chunks
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
