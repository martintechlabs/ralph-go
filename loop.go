package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// getUncommittedFiles gets list of uncommitted files
func getUncommittedFiles() []string {
	cmd := exec.Command("git", "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return []string{}
	}
	
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var result []string
	for _, line := range lines {
		if len(line) > 3 {
			// Git status format: "XY filename"
			filename := strings.TrimSpace(line[3:])
			if filename != "" {
				result = append(result, filename)
			}
		}
	}
	return result
}

// countIncompletePRDTasks parses .ralph/PRD.md and counts incomplete tasks (tasks with "- [ ]")
func countIncompletePRDTasks() (int, error) {
	content, err := os.ReadFile(".ralph/PRD.md")
	if err != nil {
		return 0, err
	}

	// Count lines matching "- [ ]" pattern (incomplete tasks)
	count := 0
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.Contains(line, "- [ ]") {
			count++
		}
	}
	return count, nil
}

// executeRalphWorkflow runs the main Ralph workflow loop
// Returns (completed bool, error) where completed=true means PRD was completed successfully
// Parameters:
//   - maxIterations: maximum number of iterations to run
//   - progressCallback: optional callback function called after each iteration (for manager mode)
func executeRalphWorkflow(maxIterations int, progressCallback ProgressCallback) (bool, error) {
	// Verify required files exist
	for _, filename := range RequiredFiles {
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			return false, fmt.Errorf("required file %s not found", filename)
		}
	}

	// Main loop
	for i := 1; i <= maxIterations; i++ {
		fmt.Printf("🔄 Iteration %d/%d\n", i, maxIterations)

		// Save state at iteration start
		state := &State{
			Iteration:            i,
			MaxIterations:        maxIterations,
			CurrentStep:          1,
			LastCompletedWorkflow: 0,
		}
		if err := saveState(state); err != nil {
			return false, fmt.Errorf("error saving state: %v", err)
		}

		// Workflow 1: Plan and Implement (loops until PRD complete)
		state.CurrentStep = 1
		state.LastCompletedWorkflow = 0
		if err := saveState(state); err != nil {
			return false, fmt.Errorf("error saving state: %v", err)
		}

		// Loop Workflow 1 until PRD is complete
		for {
			result, err := workflow1PlanAndImplement(i, maxIterations)
			if err != nil {
				return false, fmt.Errorf("error in Workflow 1: %v", err)
			}

			if result.Blocked {
				return false, fmt.Errorf("blocked during planning")
			}

			if result.Complete {
				fmt.Printf("✅ PRD complete!\n")
				break // Exit Workflow 1 loop
			}

			// Continue Workflow 1 loop (plan and implement again)
		}

		state.LastCompletedWorkflow = 1
		if err := saveState(state); err != nil {
			return false, fmt.Errorf("error saving state: %v", err)
		}

		// Workflow 2: Clean up and Review
		// Always run if we skip workflow 1 (resume) or after completing workflow 1
		state.CurrentStep = 2
		state.LastCompletedWorkflow = 1
		if err := saveState(state); err != nil {
			return false, fmt.Errorf("error saving state: %v", err)
		}

		// Count incomplete tasks before Workflow 2
		tasksBefore, _ := countIncompletePRDTasks()

		// Run Workflow 2
		err := workflow2CleanupAndReview(i, maxIterations)
		if err != nil {
			return false, fmt.Errorf("error in Workflow 2: %v", err)
		}

		state.LastCompletedWorkflow = 2
		if err := saveState(state); err != nil {
			return false, fmt.Errorf("error saving state: %v", err)
		}

		// Gather progress information and call callback (for manager mode)
		if progressCallback != nil {
			progress := IterationProgress{
				Iteration:     i,
				MaxIterations: maxIterations,
			}

			// Determine which workflows were completed
			var stepsCompleted []string
			if state.LastCompletedWorkflow >= 1 {
				stepsCompleted = append(stepsCompleted, "Plan and Implement")
			}
			if state.LastCompletedWorkflow >= 2 {
				stepsCompleted = append(stepsCompleted, "Clean up and Review")
			}
			progress.StepsCompleted = stepsCompleted

			// Get commit information
			commitMsg := getLastCommitMessage()
			if commitMsg != "" {
				progress.CommitMessage = commitMsg
				progress.FilesChanged = getChangedFiles()
			} else {
				// No commit yet, check for uncommitted changes
				progress.FilesChanged = getUncommittedFiles()
			}

			// Call the progress callback
			if err := progressCallback(progress); err != nil {
				// Log error but don't fail the iteration
				fmt.Printf("⚠️  Warning: progress callback failed: %v\n", err)
			}
		}

		// Count incomplete tasks after Workflow 2
		tasksAfter, _ := countIncompletePRDTasks()

		// If new tasks were created, continue loop (go back to Workflow 1)
		if tasksAfter > tasksBefore {
			fmt.Printf("📝 Workflow 2 created %d new PRD task(s), continuing loop...\n", tasksAfter-tasksBefore)
			continue
		}

		// No new tasks, PRD complete
		clearState()
		return true, nil
	}

	// Iteration limit reached
	clearState()
	return false, nil
}
