package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type ClaudeResult struct {
	Output   string
	Success  bool
	Blocked  bool
	Complete bool
}

func runClaude(timeoutSeconds int, systemPrompt string, prompt string) (*ClaudeResult, error) {
	ctx, cancel := contextWithTimeout(timeoutSeconds)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude",
		"--system-prompt", systemPrompt,
		"--dangerously-skip-permissions",
		"--no-session-persistence",
		"-p", prompt)

	// Capture output but don't print here (will be printed in executeStepWithRetry)
	output, err := cmd.CombinedOutput()
	result := &ClaudeResult{
		Output:  string(output),
		Success: err == nil,
	}

	if err != nil {
		// Check if it was a timeout
		if ctx.Err() == context.DeadlineExceeded {
			result.Success = false
			return result, fmt.Errorf("timeout after %d seconds", timeoutSeconds)
		}
		// For other errors, still return the result with output
		return result, err
	}

	// Check for special markers
	result.Blocked = strings.Contains(result.Output, "<promise>BLOCKED</promise>")
	result.Complete = strings.Contains(result.Output, "<promise>COMPLETE</promise>")

	return result, nil
}

func contextWithTimeout(seconds int) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), time.Duration(seconds)*time.Second)
}
