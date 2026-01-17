package main

// Timeout configuration (in seconds)
const (
	TimeoutPlanning        = 1800 // 30 minutes for planning
	TimeoutImplementation  = 3600 // 60 minutes for implementation
	TimeoutCleanup         = 900  // 15 minutes for cleanup
	TimeoutSelfImprovement = 1800 // 30 minutes for self-improvement analysis
	TimeoutCommit          = 300  // 5 minutes for commit
)

const (
	MaxRetries = 3
	StateFile  = ".ralph/ralph-state.txt"
)

// Required files
var RequiredFiles = []string{
	".ralph/PRD.md",
}
