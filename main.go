package main

import (
	"fmt"
	"os"
	"strings"
)

func printHelp() {
	fmt.Println("Ralph - Automated Development Loop")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Printf("  %s <iterations>\n", os.Args[0])
	fmt.Printf("  %s --export-prompts\n", os.Args[0])
	fmt.Printf("  %s --init [description]\n", os.Args[0])
	fmt.Printf("  %s --manager <config-file> <iterations>\n", os.Args[0])
	fmt.Printf("  %s --tickets <config-file>\n", os.Args[0])
	fmt.Printf("  %s --help\n", os.Args[0])
	fmt.Printf("  %s -h\n", os.Args[0])
	fmt.Printf("  %s --version\n", os.Args[0])
	fmt.Printf("  %s -v\n", os.Args[0])
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  iterations        Number of iterations to run (must be >= 1)")
	fmt.Println("  --export-prompts  Export all built-in prompts to .ralph directory for customization")
	fmt.Println("  --init            Create minimum files needed to get started (.ralph/PRD.md)")
	fmt.Println("                    If description is provided, interactively creates a PRD using Claude")
	fmt.Println("  --manager         Linear manager mode: automatically process tickets from Linear")
	fmt.Println("                    Requires config-file (TOML) and iterations parameter")
	fmt.Println("  --tickets         List all pending tickets from Linear (for testing connectivity)")
	fmt.Println("                    Requires config-file (TOML)")
	fmt.Println("  --version, -v     Display version information")
	fmt.Println()
	fmt.Println("Description:")
		fmt.Println("  Runs a Ralph loop that executes a series of development steps:")
		fmt.Println("  - Step 1: Planning")
		fmt.Println("  - Step 2: Implementation and Validation")
		fmt.Println("  - Step 3: Cleanup and Documentation")
		fmt.Println("  - Step 4: CLAUDE.md Refactoring")
		fmt.Println("  - Step 5: Self-Improvement Analysis (every 5th iteration)")
		fmt.Println("  - Step 6: Commit")
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
		fmt.Fprintf(os.Stderr, "Usage: %s <iterations> or %s --export-prompts or %s --init [description] or %s --manager <config-file> <iterations> or %s --tickets <config-file>\n", os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0])
		fmt.Fprintf(os.Stderr, "Use --help or -h for more information, or --version/-v for version\n")
		os.Exit(1)
	}

	// Check for help flag
	if os.Args[1] == "--help" || os.Args[1] == "-h" {
		printHelp()
		os.Exit(0)
	}

	// Check for version flag
	if os.Args[1] == "--version" || os.Args[1] == "-v" {
		fmt.Printf("Ralph version %s\n", Version)
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

	// Check for init flag
	if os.Args[1] == "--init" {
		// Check if description parameter is provided
		description := ""
		if len(os.Args) > 2 {
			// Join all remaining args as the description (handles multi-word descriptions)
			description = strings.Join(os.Args[2:], " ")
		}
		if err := initProject(description); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Error initializing project: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Check for manager flag
	if os.Args[1] == "--manager" {
		if len(os.Args) < 4 {
			fmt.Fprintf(os.Stderr, "Usage: %s --manager <config-file> <iterations>\n", os.Args[0])
			fmt.Fprintf(os.Stderr, "  config-file: Path to Linear config TOML file\n")
			fmt.Fprintf(os.Stderr, "  iterations:  Number of iterations to run per ticket (must be >= 1)\n")
			os.Exit(1)
		}

		configFile := os.Args[2]
		var iterations int
		if _, err := fmt.Sscanf(os.Args[3], "%d", &iterations); err != nil || iterations < 1 {
			fmt.Fprintf(os.Stderr, "Error: invalid iterations value: %s (must be >= 1)\n", os.Args[3])
			os.Exit(1)
		}

		if err := runManagerMode(configFile, iterations); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Manager mode error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Check for tickets flag (test Linear connectivity)
	if os.Args[1] == "--tickets" {
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: %s --tickets <config-file>\n", os.Args[0])
			fmt.Fprintf(os.Stderr, "  config-file: Path to Linear config TOML file\n")
			os.Exit(1)
		}

		configFile := os.Args[2]
		if err := listPendingTickets(configFile); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Error listing tickets: %v\n", err)
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

	// Use shared loop function
	completed, err := executeRalphWorkflow(maxIterations, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if completed {
		fmt.Println("✅ PRD completed successfully!")
		os.Exit(0)
	} else {
		fmt.Printf("⚠️  Reached iteration limit (%d) but PRD not yet complete\n", maxIterations)
		os.Exit(1)
	}
}
