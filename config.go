package main

import "os"

// Version is the application version
const Version = "0.4.2"

// Timeout configuration (in seconds)
const (
	TimeoutPlanning        = 1800 // 30 minutes for planning
	TimeoutImplementation  = 3600 // 60 minutes for implementation
	TimeoutCleanup         = 900  // 15 minutes for cleanup
	TimeoutGuardrail       = 600  // 10 minutes for guardrail verification
	TimeoutSelfImprovement = 1800 // 30 minutes for self-improvement analysis
	TimeoutCommit          = 300  // 5 minutes for commit
	TimeoutPRDCreation     = 1800 // 30 minutes for PRD creation
	TimeoutPRDSimplification = 900 // 15 minutes for PRD simplification pass
)

const (
	MaxRetries               = 3
	TimeoutSnippetMaxLines   = 12
	TimeoutSnippetMaxChars   = 800
	StateFile                = ".ralph/ralph-state.txt"
	ManagerStateFile  = ".ralph/manager-state.txt"
	LinearAPIEndpoint = "https://api.linear.app/graphql"
)

// GuardrailsFile is the project-root file that defines guardrails (optional). When present, Ralph verifies implementations against it.
const GuardrailsFile = "GUARDRAILS.md"

// Required files
var RequiredFiles = []string{
	".ralph/PRD.md",
}

// guardrailsExists returns true if GUARDRAILS.md exists in the project root.
func guardrailsExists() bool {
	_, err := os.Stat(GuardrailsFile)
	return err == nil
}
