package main

import (
	"context"
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
		"-p", prompt)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %v", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %v", err)
	}

	// Plain text output: read stdout and stderr
	stdoutBytes, _ := io.ReadAll(stdout)
	stderrBytes, _ := io.ReadAll(stderr)
	stdoutStr := strings.TrimSpace(string(stdoutBytes))
	stderrStr := strings.TrimSpace(string(stderrBytes))

	// Echo stdout to the user
	if stdoutStr != "" {
		fmt.Println(stdoutStr)
	}

	err = cmd.Wait()

	result := &ClaudeResult{
		Output:  stdoutStr,
		Success: err == nil,
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.Success = false
			details := extractErrorDetails(stderrStr, "", stdoutStr, err)
			details.Category = "timeout"
			details.Message = fmt.Sprintf("Request timeout after %d seconds", timeoutSeconds)
			details.Suggestion = "The request took too long to complete. This may be due to a slow connection, API issues, or a very complex request."
			return result, formatClaudeError(details)
		}
		details := extractErrorDetails(stderrStr, "", stdoutStr, err)
		return result, formatClaudeError(details)
	}

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
	FullOutput  string // Full command output for debugging
	ExitCode    int    // Exit code if available
}

// extractErrorDetails parses stderr and identifies error types
func extractErrorDetails(stderr string, streamError string, fullOutput string, rawError error) *ErrorDetails {
	details := &ErrorDetails{
		Category:    "unknown",
		Technical:   rawError.Error(),
		StreamError: streamError,
		FullOutput:  fullOutput,
	}

	// Try to extract exit code from exec.ExitError
	if exitError, ok := rawError.(*exec.ExitError); ok {
		details.ExitCode = exitError.ExitCode()
		details.Technical = fmt.Sprintf("exit status %d", exitError.ExitCode())
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
	
	// Build a more informative suggestion
	var suggestionParts []string
	suggestionParts = append(suggestionParts, "An unexpected error occurred. Please check your Claude CLI configuration and try again.")
	
	if stderr != "" {
		suggestionParts = append(suggestionParts, fmt.Sprintf("\n   Stderr output: %s", stderr))
	}
	
	if streamError != "" {
		suggestionParts = append(suggestionParts, fmt.Sprintf("\n   Stream error: %s", streamError))
	}
	
	if details.ExitCode != 0 {
		suggestionParts = append(suggestionParts, fmt.Sprintf("\n   Exit code: %d", details.ExitCode))
	}
	
	// Include relevant parts of fullOutput if it contains useful info
	if fullOutput != "" {
		// Look for error patterns in fullOutput
		outputLines := strings.Split(fullOutput, "\n")
		var errorLines []string
		for _, line := range outputLines {
			lowerLine := strings.ToLower(line)
			if strings.Contains(lowerLine, "error") ||
				strings.Contains(lowerLine, "failed") ||
				strings.Contains(lowerLine, "invalid") ||
				strings.Contains(lowerLine, "unauthorized") {
				errorLines = append(errorLines, line)
				if len(errorLines) >= 3 { // Limit to first 3 error lines
					break
				}
			}
		}
		if len(errorLines) > 0 {
			suggestionParts = append(suggestionParts, "\n   Command output (relevant lines):")
			for _, line := range errorLines {
				suggestionParts = append(suggestionParts, fmt.Sprintf("     %s", line))
			}
		}
	}
	
	details.Suggestion = strings.Join(suggestionParts, "")
	return details
}

// formatClaudeError formats a user-friendly error message from error details
func formatClaudeError(details *ErrorDetails) error {
	var msg strings.Builder
	msg.WriteString(details.Message)
	
	if details.Suggestion != "" {
		// Add suggestion - it already has proper formatting
		msg.WriteString("\n")
		msg.WriteString(details.Suggestion)
	}
	
	// Include technical details if they're different from what's already shown
	if details.Technical != "" && 
		!strings.Contains(strings.ToLower(details.Suggestion), strings.ToLower(details.Technical)) &&
		!strings.Contains(strings.ToLower(details.Technical), strings.ToLower(details.Message)) {
		msg.WriteString("\n   Technical details: ")
		msg.WriteString(details.Technical)
	}
	
	return fmt.Errorf("%s", msg.String())
}
