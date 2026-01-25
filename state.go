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
	Iteration         int
	MaxIterations     int
	CurrentStep       int
	LastCompletedStep int
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
		case "last_completed_step":
			state.LastCompletedStep, _ = strconv.Atoi(value)
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
	fmt.Fprintf(file, "last_completed_step=%d\n", state.LastCompletedStep)

	return nil
}

func clearState() error {
	if _, err := os.Stat(StateFile); os.IsNotExist(err) {
		return nil
	}
	return os.Remove(StateFile)
}

func detectResume(maxIterations int) (*State, int, error) {
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

	// Determine resume step based on PLAN.md existence
	var resumeStep int
	var stepName string
	if _, err := os.Stat("PLAN.md"); err == nil {
		resumeStep = 2
		stepName = "Step 2 (Implementation)"
	} else if state.LastCompletedStep >= 2 {
		resumeStep = 3
		stepName = "Step 3 (Cleanup)"
	} else {
		resumeStep = 1
		stepName = "Step 1 (Planning)"
	}

	// Prompt user
	fmt.Println("🔄 Resume detected:")
	fmt.Printf("   Iteration: %d/%d\n", state.Iteration, state.MaxIterations)
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

	return state, resumeStep, nil
}
