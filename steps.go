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

func planning(iteration, maxIterations int) (*ClaudeResult, error) {
	systemPrompt, err := getSystemPrompt()
	if err != nil {
		return nil, fmt.Errorf("failed to get system prompt: %v", err)
	}

	prompt := getStepPrompt(1)

	return executeStepWithRetry(1, "📋 Planning...", TimeoutPlanning, systemPrompt, prompt)
}

func implementation(iteration, maxIterations int) (*ClaudeResult, error) {
	systemPrompt, err := getSystemPrompt()
	if err != nil {
		return nil, fmt.Errorf("failed to get system prompt: %v", err)
	}

	prompt := getStepPrompt(2)

	return executeStepWithRetry(2, "🔨 Implementation and Validation...", TimeoutImplementation, systemPrompt, prompt)
}

func cleanup(iteration, maxIterations int) (*ClaudeResult, error) {
	systemPrompt, err := getSystemPrompt()
	if err != nil {
		return nil, fmt.Errorf("failed to get system prompt: %v", err)
	}

	prompt := getStepPrompt(3)

	return executeStepWithRetry(3, "🧹 Cleanup and Documentation...", TimeoutCleanup, systemPrompt, prompt)
}

func agentsRefactor(iteration, maxIterations int) (*ClaudeResult, error) {
	systemPrompt, err := getSystemPrompt()
	if err != nil {
		return nil, fmt.Errorf("failed to get system prompt: %v", err)
	}

	prompt := getStepPrompt(4)

	return executeStepWithRetry(4, "📝 Agents Refactor (CLAUDE.md)...", TimeoutCleanup, systemPrompt, prompt)
}

func selfImprovement(iteration, maxIterations int) (*ClaudeResult, error) {
	systemPrompt, err := getSystemPrompt()
	if err != nil {
		return nil, fmt.Errorf("failed to get system prompt: %v", err)
	}

	prompt := getStepPrompt(5)

	return executeStepWithRetry(5, fmt.Sprintf("🔍 Self-Improvement (iteration %d)...", iteration), TimeoutSelfImprovement, systemPrompt, prompt)
}

func commit(iteration, maxIterations int) (*ClaudeResult, error) {
	systemPrompt, err := getSystemPrompt()
	if err != nil {
		return nil, fmt.Errorf("failed to get system prompt: %v", err)
	}

	prompt := getStepPrompt(6)

	return executeStepWithRetry(6, "💾 Commit...", TimeoutCommit, systemPrompt, prompt)
}

func guardrailVerify(iteration, maxIterations int) (*ClaudeResult, error) {
	systemPrompt, err := getSystemPrompt()
	if err != nil {
		return nil, fmt.Errorf("failed to get system prompt: %v", err)
	}

	prompt := getGuardrailVerifyPrompt()

	return executeStepWithRetry(0, "🛡️ Guardrail verification...", TimeoutGuardrail, systemPrompt, prompt)
}

// workflow1PlanAndImplement runs planning, implementation, and commit in sequence
// Returns the result from planning step (which contains Complete flag)
func workflow1PlanAndImplement(iteration, maxIterations int) (*ClaudeResult, error) {
	// Planning
	result, err := planning(iteration, maxIterations)
	if err != nil {
		return nil, err
	}

	// If blocked or complete, return early
	if result.Blocked || result.Complete {
		return result, nil
	}

	// Implementation
	implResult, err := implementation(iteration, maxIterations)
	if err != nil {
		return nil, err
	}
	if implResult.Blocked {
		result.Blocked = true
		return result, nil
	}

	// Guardrail verification (if GUARDRAILS.md exists)
	if guardrailsExists() {
		_, err = guardrailVerify(iteration, maxIterations)
		if err != nil {
			return nil, err
		}
	}

	// Cleanup (remove PLAN.md, update PROGRESS/CLAUDE/README)
	_, err = cleanup(iteration, maxIterations)
	if err != nil {
		return nil, err
	}

	// Commit (update PRD task complete, then stage and commit)
	_, err = commit(iteration, maxIterations)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// workflow2CleanupAndReview runs refactoring and self-improvement in sequence
func workflow2CleanupAndReview(iteration, maxIterations int) error {
	// CLAUDE.md Refactoring
	_, err := agentsRefactor(iteration, maxIterations)
	if err != nil {
		return err
	}

	// Self-Improvement
	_, err = selfImprovement(iteration, maxIterations)
	if err != nil {
		return err
	}

	return nil
}
