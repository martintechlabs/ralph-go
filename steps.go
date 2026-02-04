package main

import (
	"fmt"
	"os"
	"strings"
)

func readFileContent(filename string) (string, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func executeStepWithRetry(stepNum int, stepName string, timeout int, systemPrompt string, prompt string) (*ClaudeResult, error) {
	for attempt := 0; attempt < MaxRetries; attempt++ {
		if attempt > 0 {
			fmt.Printf("\n🔄 Retrying %s (attempt %d/%d)...\n", stepName, attempt+1, MaxRetries)
		} else {
			fmt.Printf("\n%s (timeout: %ds)\n", stepName, timeout)
		}

		result, err := runClaude(timeout, systemPrompt, prompt)

		// Output is already streamed and printed in runClaude, add a newline at the end
		if result != nil {
			fmt.Print("\n")
		}

		if err != nil {
			// Check for timeout errors (they may be formatted differently now)
			errStr := err.Error()
			if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "Request timeout") {
				if attempt >= MaxRetries-1 {
					fmt.Printf("⏱️  %s timed out after %d attempts\n", stepName, MaxRetries)
					return result, err
				}
				fmt.Printf("⏱️  %s timed out after %ds, will retry...\n", stepName, timeout)
				continue
			}
			// Display formatted error message (already includes user-friendly formatting)
			fmt.Printf("❌ %s failed:\n%s\n", stepName, err.Error())
			return result, err
		}

		if result.Success {
			return result, nil
		}
	}

	return nil, fmt.Errorf("%s failed after %d attempts", stepName, MaxRetries)
}

func step1Planning(iteration, maxIterations int) (*ClaudeResult, error) {
	systemPrompt, err := getSystemPrompt()
	if err != nil {
		return nil, fmt.Errorf("failed to get system prompt: %v", err)
	}

	prompt := getStepPrompt(1)

	return executeStepWithRetry(1, "📋 Step Planning...", TimeoutPlanning, systemPrompt, prompt)
}

func step2Implementation(iteration, maxIterations int) (*ClaudeResult, error) {
	systemPrompt, err := getSystemPrompt()
	if err != nil {
		return nil, fmt.Errorf("failed to get system prompt: %v", err)
	}

	prompt := getStepPrompt(2)

	return executeStepWithRetry(2, "🔨 Step Implementation and Validation...", TimeoutImplementation, systemPrompt, prompt)
}

func step3Cleanup(iteration, maxIterations int) (*ClaudeResult, error) {
	systemPrompt, err := getSystemPrompt()
	if err != nil {
		return nil, fmt.Errorf("failed to get system prompt: %v", err)
	}

	prompt := getStepPrompt(3)

	return executeStepWithRetry(3, "🧹 Step Cleanup and Documentation...", TimeoutCleanup, systemPrompt, prompt)
}

func step4AgentsRefactor(iteration, maxIterations int) (*ClaudeResult, error) {
	systemPrompt, err := getSystemPrompt()
	if err != nil {
		return nil, fmt.Errorf("failed to get system prompt: %v", err)
	}

	prompt := getStepPrompt(4)

	return executeStepWithRetry(4, "📝 Step CLAUDE.md Refactoring...", TimeoutCleanup, systemPrompt, prompt)
}

func step5SelfImprovement(iteration, maxIterations int) (*ClaudeResult, error) {
	systemPrompt, err := getSystemPrompt()
	if err != nil {
		return nil, fmt.Errorf("failed to get system prompt: %v", err)
	}

	prompt := getStepPrompt(5)

	return executeStepWithRetry(5, fmt.Sprintf("🔍 Step Self-Improvement Analysis (iteration %d)...", iteration), TimeoutSelfImprovement, systemPrompt, prompt)
}

func step6Commit(iteration, maxIterations int) (*ClaudeResult, error) {
	systemPrompt, err := getSystemPrompt()
	if err != nil {
		return nil, fmt.Errorf("failed to get system prompt: %v", err)
	}

	prompt := getStepPrompt(6)

	return executeStepWithRetry(6, "💾 Step Commit...", TimeoutCommit, systemPrompt, prompt)
}

// workflow1PlanAndImplement runs planning, implementation, and commit in sequence
// Returns the result from planning step (which contains Complete flag)
func workflow1PlanAndImplement(iteration, maxIterations int) (*ClaudeResult, error) {
	// Planning
	result, err := step1Planning(iteration, maxIterations)
	if err != nil {
		return nil, err
	}

	// If blocked or complete, return early
	if result.Blocked || result.Complete {
		return result, nil
	}

	// Implementation
	implResult, err := step2Implementation(iteration, maxIterations)
	if err != nil {
		return nil, err
	}
	if implResult.Blocked {
		result.Blocked = true
		return result, nil
	}

	// Commit
	_, err = step6Commit(iteration, maxIterations)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// workflow2CleanupAndReview runs cleanup, refactoring, and self-improvement in sequence
func workflow2CleanupAndReview(iteration, maxIterations int) error {
	// Cleanup
	_, err := step3Cleanup(iteration, maxIterations)
	if err != nil {
		return err
	}

	// CLAUDE.md Refactoring
	_, err = step4AgentsRefactor(iteration, maxIterations)
	if err != nil {
		return err
	}

	// Self-Improvement
	_, err = step5SelfImprovement(iteration, maxIterations)
	if err != nil {
		return err
	}

	return nil
}
