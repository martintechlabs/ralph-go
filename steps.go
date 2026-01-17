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

		// Print output (like tee in bash script)
		if result != nil && result.Output != "" {
			fmt.Print(result.Output)
		}

		if err != nil {
			if strings.Contains(err.Error(), "timeout") {
				if attempt >= MaxRetries-1 {
					fmt.Printf("⏱️  %s timed out after %d attempts\n", stepName, MaxRetries)
					return result, err
				}
				fmt.Printf("⏱️  %s timed out after %ds, will retry...\n", stepName, timeout)
				continue
			}
			fmt.Printf("❌ %s failed: %v\n", stepName, err)
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

	return executeStepWithRetry(1, "📋 Step 1: Planning...", TimeoutPlanning, systemPrompt, prompt)
}

func step2Implementation(iteration, maxIterations int) (*ClaudeResult, error) {
	systemPrompt, err := getSystemPrompt()
	if err != nil {
		return nil, fmt.Errorf("failed to get system prompt: %v", err)
	}

	prompt := getStepPrompt(2)

	return executeStepWithRetry(2, "🔨 Step 2: Implementation and Validation...", TimeoutImplementation, systemPrompt, prompt)
}

func step3Cleanup(iteration, maxIterations int) (*ClaudeResult, error) {
	systemPrompt, err := getSystemPrompt()
	if err != nil {
		return nil, fmt.Errorf("failed to get system prompt: %v", err)
	}

	prompt := getStepPrompt(3)

	return executeStepWithRetry(3, "🧹 Step 3: Cleanup and Documentation...", TimeoutCleanup, systemPrompt, prompt)
}

func step4SelfImprovement(iteration, maxIterations int) (*ClaudeResult, error) {
	systemPrompt, err := getSystemPrompt()
	if err != nil {
		return nil, fmt.Errorf("failed to get system prompt: %v", err)
	}

	prompt := getStepPrompt(4)

	return executeStepWithRetry(4, fmt.Sprintf("🔍 Step 4: Self-Improvement Analysis (iteration %d)...", iteration), TimeoutSelfImprovement, systemPrompt, prompt)
}

func step5Commit(iteration, maxIterations int) (*ClaudeResult, error) {
	systemPrompt, err := getSystemPrompt()
	if err != nil {
		return nil, fmt.Errorf("failed to get system prompt: %v", err)
	}

	prompt := getStepPrompt(5)

	return executeStepWithRetry(5, "💾 Step 5: Commit...", TimeoutCommit, systemPrompt, prompt)
}
