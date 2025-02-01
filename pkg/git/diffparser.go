package git

import (
	"os/exec"
	"strings"
)

// DiffChunk represents one contiguous set of changed lines in the diff
type DiffChunk struct {
	FilePath   string
	HunkHeader string
	Lines      []string
}

// ParseDiffToChunks processes a unified diff string into a list of DiffChunks.
// This is a naive parser that assumes the diff is in "git diff --staged" format.
func ParseDiffToChunks(diff string) ([]DiffChunk, error) {
	lines := strings.Split(diff, "\n")
	var chunks []DiffChunk

	var currentChunk *DiffChunk
	var currentFile string
	var inHunk bool

	for _, line := range lines {
		// Detect start of a new file
		if strings.HasPrefix(line, "diff --git ") {
			// finalize any ongoing chunk
			if currentChunk != nil {
				chunks = append(chunks, *currentChunk)
				currentChunk = nil
			}
			// parse file path from line if possible
			file := parseFilePath(line)
			if file != "" {
				currentFile = file
			}
			inHunk = false
			continue
		}

		// Detect start of a hunk
		if strings.HasPrefix(line, "@@ ") {
			// finalize any existing chunk
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
			// lines in the current hunk
			currentChunk.Lines = append(currentChunk.Lines, line)
		}
	}

	// finalize last chunk
	if currentChunk != nil {
		chunks = append(chunks, *currentChunk)
	}

	return chunks, nil
}

// parseFilePath tries to parse a file path from a "diff --git a/xxx b/xxx" line
func parseFilePath(diffLine string) string {
	// example: diff --git a/pkg/git/git.go b/pkg/git/git.go
	// we want to extract: pkg/git/git.go
	parts := strings.Split(diffLine, " ")
	if len(parts) < 4 {
		return ""
	}
	// usually parts[2] = a/xxx, parts[3] = b/xxx
	aPath := strings.TrimPrefix(parts[2], "a/")
	bPath := strings.TrimPrefix(parts[3], "b/")
	if aPath == bPath {
		return aPath
	}
	return bPath
}

// GetHeadCommitMessage retrieves the HEAD commit message
func GetHeadCommitMessage() (string, error) {
	out, err := run("git", "log", "-1", "--pretty=%B")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// run executes the given command and returns its output as a string
func run(cmdName string, args ...string) (string, error) {
	cmd := exec.Command(cmdName, args...)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}
