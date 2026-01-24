package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

// Stream message types from Claude's JSON stream format
type streamMessage struct {
	Type    string `json:"type"`
	Message struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"message"`
	Result string `json:"result"`
}

func runClaude(timeoutSeconds int, systemPrompt string, prompt string) (*ClaudeResult, error) {
	ctx, cancel := contextWithTimeout(timeoutSeconds)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude",
		"--system-prompt", systemPrompt,
		"--dangerously-skip-permissions",
		"--no-session-persistence",
		"--output-format", "stream-json",
		"-p", prompt)

	// Get stdout pipe for streaming
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %v", err)
	}

	// Get stderr pipe to capture errors
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %v", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %v", err)
	}

	// Read and process streaming output
	var fullOutput strings.Builder
	scanner := bufio.NewScanner(stdout)

	// Process stdout line by line
	for scanner.Scan() {
		line := scanner.Text()

		// Only process lines that look like JSON (start with {)
		// This matches the bash script's grep '^{' filter
		if strings.HasPrefix(line, "{") {
			fullOutput.WriteString(line)
			fullOutput.WriteString("\n")

			// Parse JSON stream message
			var msg streamMessage
			if err := json.Unmarshal([]byte(line), &msg); err == nil {
				// Extract and print streaming text from assistant messages
				// This matches: select(.type == "assistant").message.content[]? | select(.type == "text").text
				if msg.Type == "assistant" {
					for _, content := range msg.Message.Content {
						if content.Type == "text" && content.Text != "" {
							// Print streaming text (replace \n with \r\n for proper display)
							// This matches: gsub("\n"; "\r\n")
							text := strings.ReplaceAll(content.Text, "\n", "\r\n")
							fmt.Print(text)
						}
					}
				}

				// Check for final result
				// This matches: select(.type == "result").result
				if msg.Type == "result" && msg.Result != "" {
					fullOutput.WriteString(msg.Result)
					fullOutput.WriteString("\n")
				}
			}
		} else {
			// Non-JSON lines (like verbose output) - still capture them
			fullOutput.WriteString(line)
			fullOutput.WriteString("\n")
		}
	}

	// Check for scanner errors
	if err := scanner.Err(); err != nil {
		// Continue anyway, we'll capture what we got
		fullOutput.WriteString(fmt.Sprintf("\nScanner error: %v\n", err))
	}

	// Read stderr
	stderrOutput, _ := io.ReadAll(stderr)
	if len(stderrOutput) > 0 {
		fullOutput.WriteString("STDERR: ")
		fullOutput.Write(stderrOutput)
		fullOutput.WriteString("\n")
	}

	// Wait for command to complete
	err = cmd.Wait()

	outputStr := fullOutput.String()
	result := &ClaudeResult{
		Output:  outputStr,
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

	// Check for special markers in the full output
	result.Blocked = strings.Contains(result.Output, "<promise>BLOCKED</promise>")
	result.Complete = strings.Contains(result.Output, "<promise>COMPLETE</promise>")

	return result, nil
}

func contextWithTimeout(seconds int) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), time.Duration(seconds)*time.Second)
}
