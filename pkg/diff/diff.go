package diff

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Runner handles running diff commands.
type Runner interface {
	Run(ctx context.Context, command string) (string, error)
	SplitIntoChunks(diff string, maxChunkSize int) []string
}

type runner struct{}

// NewRunner creates a new diff runner.
func NewRunner() Runner {
	return &runner{}
}

// Run executes a diff command and returns its output.
func (r *runner) Run(ctx context.Context, command string) (string, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("diff command failed with stderr: %s: %w", exitErr.Stderr, err)
		}
		return "", fmt.Errorf("failed to execute diff command: %w", err)
	}

	return string(output), nil
}

// SplitIntoChunks splits the diff into chunks not exceeding maxChunkSize.
func (r *runner) SplitIntoChunks(diff string, maxChunkSize int) []string {
	if len(diff) <= maxChunkSize {
		return []string{diff}
	}

	var chunks []string
	lines := strings.Split(diff, "\n")
	currentChunk := strings.Builder{}

	for _, line := range lines {
		if currentChunk.Len()+len(line)+1 > maxChunkSize {
			if currentChunk.Len() > 0 {
				chunks = append(chunks, currentChunk.String())
				currentChunk.Reset()
			}
		}
		currentChunk.WriteString(line)
		currentChunk.WriteString("\n")
	}

	if currentChunk.Len() > 0 {
		chunks = append(chunks, currentChunk.String())
	}

	return chunks
}
