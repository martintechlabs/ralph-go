package main

import (
	"fmt"
	"os"
)

func printHelp() {
	fmt.Println("Ralph - Automated Development Loop")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Printf("  %s <iterations>\n", os.Args[0])
	fmt.Printf("  %s --export-prompts\n", os.Args[0])
	fmt.Printf("  %s --help\n", os.Args[0])
	fmt.Printf("  %s -h\n", os.Args[0])
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  iterations        Number of iterations to run (must be >= 1)")
	fmt.Println("  --export-prompts  Export all built-in prompts to .ralph directory for customization")
	fmt.Println()
	fmt.Println("Description:")
	fmt.Println("  Runs a Ralph loop that executes a series of development steps:")
	fmt.Println("  - Step 1: Planning")
	fmt.Println("  - Step 2: Implementation and Validation")
	fmt.Println("  - Step 3: Cleanup and Documentation")
	fmt.Println("  - Step 4: Self-Improvement Analysis (every 5th iteration)")
	fmt.Println("  - Step 5: Commit")
	fmt.Println()
	fmt.Println("Features:")
	fmt.Println("  - Automatic resume from last checkpoint if interrupted")
	fmt.Println("  - State persistence across runs")
	fmt.Println("  - Configurable via .ralph directory")
	fmt.Println()
	fmt.Println("Required Files:")
	fmt.Println("  - .ralph/PRD.md")
	fmt.Println()
	fmt.Println("Prompt Customization:")
	fmt.Println("  Use --export-prompts to export built-in prompts to .ralph directory.")
	fmt.Println("  Customize prompts by editing files in .ralph/")
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <iterations> or %s --export-prompts\n", os.Args[0], os.Args[0])
		fmt.Fprintf(os.Stderr, "Use --help or -h for more information\n")
		os.Exit(1)
	}

	// Check for help flag
	if os.Args[1] == "--help" || os.Args[1] == "-h" {
		printHelp()
		os.Exit(0)
	}

	// Check for export-prompts flag
	if os.Args[1] == "--export-prompts" {
		if err := exportPrompts(); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Error exporting prompts: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	var maxIterations int
	if _, err := fmt.Sscanf(os.Args[1], "%d", &maxIterations); err != nil || maxIterations < 1 {
		fmt.Fprintf(os.Stderr, "Error: invalid iterations value: %s\n", os.Args[1])
		os.Exit(1)
	}

	// Use current working directory (where the command is run from)
	// This matches the bash script behavior of using the script's directory
	scriptDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to get current directory: %v\n", err)
		os.Exit(1)
	}

	// Verify required files exist
	for _, filename := range RequiredFiles {
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "❌ Error: %s not found in %s\n", filename, scriptDir)
			os.Exit(1)
		}
	}

	// Resume detection
	startIteration := 1
	resumeStep := 0
	resumeState, resumeStepNum, err := detectResume(maxIterations)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error detecting resume state: %v\n", err)
		os.Exit(1)
	}

	if resumeState != nil {
		startIteration = resumeState.Iteration
		resumeStep = resumeStepNum
		fmt.Printf("✅ Resuming from iteration %d, step %d\n", startIteration, resumeStep)
	} else {
		fmt.Println("🚀 Starting fresh")
	}

	// Main loop
	for i := startIteration; i <= maxIterations; i++ {
		fmt.Printf("🔄 Iteration %d/%d\n", i, maxIterations)

		// Save state at iteration start
		state := &State{
			Iteration:         i,
			MaxIterations:     maxIterations,
			CurrentStep:       1,
			LastCompletedStep: 0,
		}
		if err := saveState(state); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving state: %v\n", err)
		}

		// Determine if we should skip to a later step (resume)
		skipToStep := 0
		if i == startIteration && resumeStep > 1 {
			skipToStep = resumeStep
		}

		// Step 1: Planning
		if skipToStep <= 1 {
			state.CurrentStep = 1
			state.LastCompletedStep = 0
			if err := saveState(state); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving state: %v\n", err)
			}

			result, err := step1Planning(i, maxIterations)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error in Step 1: %v\n", err)
				if err := saveState(state); err != nil {
					fmt.Fprintf(os.Stderr, "Error saving state: %v\n", err)
				}
				os.Exit(0)
			}

			if result.Complete {
				fmt.Printf("✅ PRD complete after %d iterations!\n", i)
				clearState()
				os.Exit(0)
			}

			if result.Blocked {
				fmt.Println("❌ Ralph is blocked during planning, please fix the blocker and run again.")
				if err := saveState(state); err != nil {
					fmt.Fprintf(os.Stderr, "Error saving state: %v\n", err)
				}
				os.Exit(0)
			}

			// Step 1 completed successfully
			state.CurrentStep = 2
			state.LastCompletedStep = 1
			if err := saveState(state); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving state: %v\n", err)
			}
		} else {
			fmt.Println("⏭️  Step 1: Skipping (resuming from later step)")
		}

		// Step 2: Implementation and Validation
		if skipToStep <= 2 {
			state.CurrentStep = 2
			state.LastCompletedStep = 1
			if err := saveState(state); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving state: %v\n", err)
			}

			result, err := step2Implementation(i, maxIterations)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error in Step 2: %v\n", err)
				if err := saveState(state); err != nil {
					fmt.Fprintf(os.Stderr, "Error saving state: %v\n", err)
				}
				os.Exit(0)
			}

			if result.Blocked {
				fmt.Println("❌ Ralph is blocked during implementation, please fix the blocker and run again.")
				if err := saveState(state); err != nil {
					fmt.Fprintf(os.Stderr, "Error saving state: %v\n", err)
				}
				os.Exit(0)
			}

			// Step 2 completed successfully
			state.CurrentStep = 3
			state.LastCompletedStep = 2
			if err := saveState(state); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving state: %v\n", err)
			}
		} else {
			fmt.Println("⏭️  Step 2: Skipping (resuming from later step)")
		}

		// Step 3: Cleanup and Documentation
		if skipToStep <= 3 {
			state.CurrentStep = 3
			state.LastCompletedStep = 2
			if err := saveState(state); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving state: %v\n", err)
			}

			_, err := step3Cleanup(i, maxIterations)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error in Step 3: %v\n", err)
				if err := saveState(state); err != nil {
					fmt.Fprintf(os.Stderr, "Error saving state: %v\n", err)
				}
				os.Exit(0)
			}

			// Step 3 completed successfully
			state.CurrentStep = 4
			state.LastCompletedStep = 3
			if err := saveState(state); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving state: %v\n", err)
			}
		} else {
			fmt.Println("⏭️  Step 3: Skipping (resuming from later step)")
		}

		// Step 4: Self-Improvement Analysis (every 5th iteration)
		if i%5 == 0 {
			if skipToStep <= 4 {
				state.CurrentStep = 4
				state.LastCompletedStep = 3
				if err := saveState(state); err != nil {
					fmt.Fprintf(os.Stderr, "Error saving state: %v\n", err)
				}

				_, err := step4SelfImprovement(i, maxIterations)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error in Step 4: %v\n", err)
					if err := saveState(state); err != nil {
						fmt.Fprintf(os.Stderr, "Error saving state: %v\n", err)
					}
					os.Exit(0)
				}

				// Step 4 completed successfully
				state.CurrentStep = 5
				state.LastCompletedStep = 4
				if err := saveState(state); err != nil {
					fmt.Fprintf(os.Stderr, "Error saving state: %v\n", err)
				}
			} else {
				fmt.Println("⏭️  Step 4: Skipping (resuming from later step)")
			}
		} else {
			fmt.Println("⏭️  Step 4: Skipping self-improvement analysis (runs every 5th iteration)")
		}

		// Step 5: Commit
		if skipToStep <= 5 || skipToStep == 0 {
			state.CurrentStep = 5
			state.LastCompletedStep = 4
			if err := saveState(state); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving state: %v\n", err)
			}

			_, err := step5Commit(i, maxIterations)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error in Step 5: %v\n", err)
				if err := saveState(state); err != nil {
					fmt.Fprintf(os.Stderr, "Error saving state: %v\n", err)
				}
				os.Exit(0)
			}

			// Step 5 completed successfully
			state.CurrentStep = 0
			state.LastCompletedStep = 5
			if err := saveState(state); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving state: %v\n", err)
			}
		} else {
			fmt.Println("⏭️  Step 5: Skipping (resuming from later step)")
		}

		// Clear resume step after first iteration
		if i == startIteration {
			resumeStep = 0
		}
	}

	fmt.Printf("⚠️  Reached iteration limit (%d) but PRD not yet complete\n", maxIterations)
	clearState()
	os.Exit(1)
}
