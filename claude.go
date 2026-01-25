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
	Error  struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func runClaude(timeoutSeconds int, systemPrompt string, prompt string) (*ClaudeResult, error) {
	// Check if claude command exists
	if _, err := exec.LookPath("claude"); err != nil {
		return nil, fmt.Errorf("claude command not found in PATH. Please ensure the Claude CLI is installed and available")
	}

	ctx, cancel := contextWithTimeout(timeoutSeconds)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude",
		"--system-prompt", systemPrompt,
		"--dangerously-skip-permissions",
		"--no-session-persistence",
		"--output-format", "stream-json",
		"--verbose",
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
	var fullOutput strings.Builder  // For debugging/error messages
	var textOutput strings.Builder   // For actual text content (PRD extraction)
	var streamError strings.Builder // For error messages from JSON stream
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
				// Check for error messages in the stream
				if msg.Type == "error" {
					if msg.Error.Message != "" {
						streamError.WriteString(msg.Error.Message)
						streamError.WriteString(" ")
					}
					if msg.Error.Type != "" {
						streamError.WriteString(msg.Error.Type)
						streamError.WriteString(" ")
					}
				}

				// Extract and print streaming text from assistant messages
				// This matches: select(.type == "assistant").message.content[]? | select(.type == "text").text
				if msg.Type == "assistant" {
					for _, content := range msg.Message.Content {
						if content.Type == "text" && content.Text != "" {
							// Add to text output (keep original newlines for PRD extraction)
							textOutput.WriteString(content.Text)
							// Print streaming text (replace \n with \r\n for proper display)
							// This matches: gsub("\n"; "\r\n")
							text := strings.ReplaceAll(content.Text, "\n", "\r\n")
							fmt.Print(text)
						}
					}
				}

				// Check for final result - but don't add it to textOutput as it may contain JSON metadata
				// This matches: select(.type == "result").result
				if msg.Type == "result" && msg.Result != "" {
					fullOutput.WriteString(msg.Result)
					fullOutput.WriteString("\n")
					// Don't add result to textOutput - it contains JSON metadata
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
	stderrStr := strings.TrimSpace(string(stderrOutput))
	if len(stderrStr) > 0 {
		fullOutput.WriteString("STDERR: ")
		fullOutput.WriteString(stderrStr)
		fullOutput.WriteString("\n")
	}

	// Wait for command to complete
	err = cmd.Wait()

	// Use textOutput for the main output (clean text without JSON metadata)
	// Fall back to fullOutput if textOutput is empty (for error cases)
	outputStr := textOutput.String()
	if outputStr == "" {
		outputStr = fullOutput.String()
	}
	
	result := &ClaudeResult{
		Output:  outputStr,
		Success: err == nil,
	}

	if err != nil {
		// Check if it was a timeout
		if ctx.Err() == context.DeadlineExceeded {
			result.Success = false
			details := extractErrorDetails(stderrStr, streamError.String(), err)
			details.Category = "timeout"
			details.Message = fmt.Sprintf("Request timeout after %d seconds", timeoutSeconds)
			details.Suggestion = "The request took too long to complete. This may be due to a slow connection, API issues, or a very complex request."
			return result, formatClaudeError(details)
		}
		// Extract error details and format user-friendly error message
		details := extractErrorDetails(stderrStr, streamError.String(), err)
		return result, formatClaudeError(details)
	}

	// Check for special markers in the full output
	result.Blocked = strings.Contains(result.Output, "<promise>BLOCKED</promise>")
	result.Complete = strings.Contains(result.Output, "<promise>COMPLETE</promise>")

	return result, nil
}

func contextWithTimeout(seconds int) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), time.Duration(seconds)*time.Second)
}

// ErrorDetails contains structured error information extracted from Claude CLI
type ErrorDetails struct {
	Category    string // timeout, authentication, rate_limit, api_error, network, unknown
	Message     string // User-friendly error message
	Suggestion  string // Actionable suggestion for the user
	Technical   string // Technical details for debugging
	StreamError string // Error from JSON stream if available
}

// extractErrorDetails parses stderr and identifies error types
func extractErrorDetails(stderr string, streamError string, rawError error) *ErrorDetails {
	details := &ErrorDetails{
		Category:    "unknown",
		Technical:   rawError.Error(),
		StreamError: streamError,
	}

	// Combine stderr and streamError for analysis
	combinedError := strings.ToLower(stderr + " " + streamError)

	// Check for authentication errors
	if strings.Contains(combinedError, "authentication") ||
		strings.Contains(combinedError, "unauthorized") ||
		strings.Contains(combinedError, "invalid api key") ||
		strings.Contains(combinedError, "api key") ||
		strings.Contains(combinedError, "not authenticated") ||
		strings.Contains(combinedError, "auth") {
		details.Category = "authentication"
		details.Message = "Claude API authentication error"
		details.Suggestion = "The Claude CLI could not authenticate. Please check your API key.\n   Run: claude auth login"
		return details
	}

	// Check for rate limit errors
	if strings.Contains(combinedError, "rate limit") ||
		strings.Contains(combinedError, "rate_limit") ||
		strings.Contains(combinedError, "too many requests") ||
		strings.Contains(combinedError, "429") {
		details.Category = "rate_limit"
		details.Message = "Claude API rate limit exceeded"
		details.Suggestion = "Too many requests. Please wait a few minutes and try again."
		return details
	}

	// Check for network errors
	if strings.Contains(combinedError, "network") ||
		strings.Contains(combinedError, "connection") ||
		strings.Contains(combinedError, "timeout") ||
		strings.Contains(combinedError, "dns") ||
		strings.Contains(combinedError, "refused") ||
		strings.Contains(combinedError, "no such host") {
		details.Category = "network"
		details.Message = "Network connection error"
		details.Suggestion = "Unable to connect to Claude API. Please check your internet connection and try again."
		return details
	}

	// Check for API errors
	if strings.Contains(combinedError, "api error") ||
		strings.Contains(combinedError, "bad request") ||
		strings.Contains(combinedError, "400") ||
		strings.Contains(combinedError, "500") ||
		strings.Contains(combinedError, "502") ||
		strings.Contains(combinedError, "503") ||
		strings.Contains(combinedError, "internal server error") {
		details.Category = "api_error"
		details.Message = "Claude API error"
		if stderr != "" {
			details.Suggestion = fmt.Sprintf("The Claude API returned an error. Details: %s", stderr)
		} else {
			details.Suggestion = "The Claude API returned an error. Please try again later."
		}
		return details
	}

	// Check for timeout (already handled separately, but include for completeness)
	if strings.Contains(combinedError, "timeout") || strings.Contains(combinedError, "deadline exceeded") {
		details.Category = "timeout"
		details.Message = "Request timeout"
		details.Suggestion = "The request took too long to complete. This may be due to a slow connection or API issues."
		return details
	}

	// Default: unknown error
	details.Category = "unknown"
	details.Message = "Claude command failed"
	if stderr != "" {
		details.Suggestion = fmt.Sprintf("An unexpected error occurred. Error details: %s", stderr)
	} else {
		details.Suggestion = "An unexpected error occurred. Please check your Claude CLI configuration and try again."
	}
	return details
}

// formatClaudeError formats a user-friendly error message from error details
func formatClaudeError(details *ErrorDetails) error {
	var msg strings.Builder
	msg.WriteString(details.Message)
	
	if details.Suggestion != "" {
		// Add suggestion with proper indentation
		lines := strings.Split(details.Suggestion, "\n")
		for _, line := range lines {
			if line != "" {
				msg.WriteString("\n   ")
				msg.WriteString(line)
			}
		}
	}
	
	// Include technical details if they're different from the message
	if details.Technical != "" && !strings.Contains(strings.ToLower(details.Technical), strings.ToLower(details.Message)) {
		msg.WriteString("\n   Technical details: ")
		msg.WriteString(details.Technical)
	}
	
	return fmt.Errorf("%s", msg.String())
}
