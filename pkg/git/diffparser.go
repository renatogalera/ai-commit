package git

import (
	"strings"
)

// DiffChunk represents a single "hunk" within a diff for a particular file.
type DiffChunk struct {
	FilePath   string
	HunkHeader string
	Lines      []string
}

// ParseDiffToChunks splits a unified diff into a list of DiffChunk structs.
func ParseDiffToChunks(diff string) ([]DiffChunk, error) {
	lines := strings.Split(diff, "\n")
	var chunks []DiffChunk

	var currentChunk *DiffChunk
	var currentFile string
	var inHunk bool

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			if currentChunk != nil {
				chunks = append(chunks, *currentChunk)
				currentChunk = nil
			}
			file := parseFilePath(line)
			if file != "" {
				currentFile = file
			}
			inHunk = false
			continue
		}

		if strings.HasPrefix(line, "@@ ") {
			if currentChunk != nil {
				chunks = append(chunks, *currentChunk)
			}
			currentChunk = &DiffChunk{
				FilePath:   currentFile,
				HunkHeader: line,
				Lines:      []string{},
			}
			inHunk = true
			continue
		}

		if inHunk && currentChunk != nil {
			currentChunk.Lines = append(currentChunk.Lines, line)
		}
	}

	if currentChunk != nil {
		chunks = append(chunks, *currentChunk)
	}
	return chunks, nil
}

// parseFilePath attempts to parse the file path from a diff --git line.
func parseFilePath(diffLine string) string {
	parts := strings.Split(diffLine, " ")
	if len(parts) < 4 {
		return ""
	}
	aPath := strings.TrimPrefix(parts[2], "a/")
	bPath := strings.TrimPrefix(parts[3], "b/")
	if aPath == bPath {
		return aPath
	}
	return bPath
}
