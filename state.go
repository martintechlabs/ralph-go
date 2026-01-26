package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type State struct {
	Iteration            int
	MaxIterations        int
	CurrentStep          int
	LastCompletedWorkflow int
}

func loadState() (*State, error) {
	if _, err := os.Stat(StateFile); os.IsNotExist(err) {
		return nil, nil
	}

	file, err := os.Open(StateFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	state := &State{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "iteration":
			state.Iteration, _ = strconv.Atoi(value)
		case "max_iterations":
			state.MaxIterations, _ = strconv.Atoi(value)
		case "current_step":
			state.CurrentStep, _ = strconv.Atoi(value)
		case "last_completed_workflow":
			state.LastCompletedWorkflow, _ = strconv.Atoi(value)
		case "last_completed_step":
			// Backward compatibility: handle old key name
			state.LastCompletedWorkflow, _ = strconv.Atoi(value)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return state, nil
}

func saveState(state *State) error {
	// Ensure .ralph directory exists
	dir := filepath.Dir(StateFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	file, err := os.Create(StateFile)
	if err != nil {
		return err
	}
	defer file.Close()

	fmt.Fprintf(file, "iteration=%d\n", state.Iteration)
	fmt.Fprintf(file, "max_iterations=%d\n", state.MaxIterations)
	fmt.Fprintf(file, "current_step=%d\n", state.CurrentStep)
	fmt.Fprintf(file, "last_completed_workflow=%d\n", state.LastCompletedWorkflow)

	return nil
}

func clearState() error {
	if _, err := os.Stat(StateFile); os.IsNotExist(err) {
		return nil
	}
	return os.Remove(StateFile)
}

func getStepName(step int) string {
	stepNames := map[int]string{
		1: "Workflow 1 (Plan and Implement)",
		2: "Workflow 2 (Clean up and Review)",
	}
	if name, ok := stepNames[step]; ok {
		return name
	}
	return fmt.Sprintf("Workflow %d", step)
}

func detectResume(maxIterations int) (*State, int, error) {
	return detectResumeWithPrompt(maxIterations, true)
}

func detectResumeWithPrompt(maxIterations int, interactive bool) (*State, int, error) {
	state, err := loadState()
	if err != nil {
		return nil, 0, err
	}

	if state == nil {
		return nil, 0, nil
	}

	// Validate state
	if state.Iteration == 0 || state.MaxIterations == 0 {
		fmt.Fprintf(os.Stderr, "⚠️  State file is corrupted. Starting fresh.\n")
		clearState()
		return nil, 0, nil
	}

	// Check if iteration exceeds max
	if state.Iteration > state.MaxIterations {
		fmt.Fprintf(os.Stderr, "⚠️  State file indicates iteration exceeds max. Starting fresh.\n")
		clearState()
		return nil, 0, nil
	}

	// Simplified resume logic:
	// - Track iteration and which workflow (1 or 2) we were in
	// - If LastCompletedWorkflow == 2: Workflow 2 completed, need to check for new tasks (same iteration)
	// - If LastCompletedWorkflow == 1: we were in workflow 2, resume at workflow 2 (same iteration)
	// - Otherwise (LastCompletedWorkflow == 0): we were in workflow 1, resume at beginning of workflow 1 (same iteration)
	// Note: Iterations only increment when we complete a full cycle (Workflow 1 -> Workflow 2 -> no new tasks)
	var resumeStep int
	var resumeIteration int
	if state.LastCompletedWorkflow == 2 {
		// Workflow 2 completed, need to check for new tasks in same iteration
		// Use resumeStep = 3 to indicate "skip both workflows, check tasks"
		resumeIteration = state.Iteration
		resumeStep = 3 // Special value: skip both workflows, go to task checking
	} else if state.LastCompletedWorkflow == 1 {
		// Workflow 1 complete, resume at workflow 2 of same iteration
		resumeIteration = state.Iteration
		resumeStep = 2
	} else {
		// No workflow complete yet, resume at workflow 1 of same iteration
		resumeIteration = state.Iteration
		resumeStep = 1
	}

	var stepName string
	if resumeStep == 3 {
		stepName = "Task check (after Workflow 2)"
	} else {
		stepName = getStepName(resumeStep)
	}

	// Prompt user if interactive mode
	if interactive {
		fmt.Println("🔄 Resume detected:")
		fmt.Printf("   Iteration: %d/%d\n", resumeIteration, state.MaxIterations)
		fmt.Printf("   Resume from: %s\n", stepName)
		fmt.Println()
		fmt.Print("Continue from here? (Y/n): ")

		var response string
		fmt.Scanln(&response)
		response = strings.TrimSpace(response)

		// Default to "Y" if empty, only start fresh if explicitly "n" or "N"
		if response == "n" || response == "N" {
			fmt.Println("Starting fresh...")
			clearState()
			return nil, 0, nil
		}
	} else {
		fmt.Printf("🔄 Auto-resuming from iteration %d/%d, %s\n", resumeIteration, state.MaxIterations, stepName)
	}

	return state, resumeStep, nil
}
