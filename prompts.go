package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// Built-in prompts - these are used as fallbacks if not found in .ralph directory

const BuiltInSystemPrompt = `AUTONOMOUS MODE: You are operating in fully autonomous mode.

CRITICAL RULES:

- DO NOT ask follow-up questions
- DO NOT request clarification
- DO NOT ask for confirmation before proceeding
- DO NOT ask "what should I do next?" or similar questions
- Make reasonable decisions independently and proceed immediately
- If you encounter ambiguity, use your best judgment based on the context provided
- If information is missing, make reasonable assumptions based on PRD.md, codebase patterns, and best practices
- Complete each step fully without asking if you should continue

DECISION-MAKING FRAMEWORK:
When multiple options exist, prioritize by:

1. Dependencies (work on prerequisites first)
2. Impact (higher value features first)
3. Complexity (easier tasks first if tied)
4. Codebase patterns (follow existing conventions)

EDGE CASE HANDLING:

- If a step cannot be completed: document the blocker and output <promise>BLOCKED</promise>
- If no changes are needed: proceed to next step without asking
- If commit fails (no changes): proceed anyway, do not ask what to do
- If information is ambiguous: interpret reasonably and proceed`

const BuiltInPlanningPrompt = `@.ralph/PRD.md @.ralph/PROGRESS.md @GUARDRAILS.md \
1. Review all incomplete tasks in PRD and assess their complexity (easy, medium, hard). \
2. PRIORITY: Find an incomplete task that is EASY or MEDIUM complexity. Bias towards tasks that are visiable to the user. \
3. If no easy/medium tasks exist: \
   a. Select a MEDIUM-HARD complexity task \
   b. Break it down into 3-5 smaller, manageable subtasks (each should be easy or medium complexity) \
   c. Update .ralph/PRD.md by replacing the original task with the subtasks (maintain the same checkbox format) \
   d. Select ONE of the newly created subtasks to work on \
4. Create a detailed plan for the selected task. Make sure to include vitests, detailed task breakdown and acceptance criteria. \
5. If @GUARDRAILS.md exists, ensure your plan complies with it (do not propose steps that violate those rules). \
6. Write the plan to .ralph/PLAN.md. \
ONLY WORK ON ONE TASK. \
DO NOT ask which task to work on - select one autonomously using the decision-making framework. \
Proceed immediately to planning - do not ask for confirmation. \
If PRD is complete, output <promise>COMPLETE</promise>. \
If you are blocked, output <promise>BLOCKED</promise> and explain the blocker.`

const BuiltInImplementationPrompt = `@.ralph/PRD.md @.ralph/PLAN.md @.ralph/PROGRESS.md @CLAUDE.md \
1. Pay close attention to @CLAUDE.md and follow any instructions it provides. \
2. Implement the task completely, based on .ralph/PLAN.md. \
3. Run tests and type checks. Fix ALL errors and warnings. \
4. Ensure test coverage is at least 80%. \
5. Run a code review and fix ALL issues. \
6. Verify that ALL Verification Criteria from .ralph/PRD.md for this task are met. If any criteria are not met, continue implementation until all are satisfied. \
If .ralph/PLAN.md is ambiguous, interpret it reasonably and proceed - do not ask for clarification. \
Complete the implementation fully - do not ask if you should continue or what to do next. \
If you are blocked, output <promise>BLOCKED</promise> and explain the blocker.`

const BuiltInCleanupPrompt = `@.ralph/PRD.md @.ralph/PLAN.md @.ralph/PROGRESS.md \
1. Update .ralph/PRD.md with the completed task: \
   a. CRITICAL: Verify that ALL Verification Criteria for this task are met. A task CANNOT be marked complete unless ALL verification criteria checkboxes can be checked off. \
   b. If any Verification Criteria are not met, output <promise>BLOCKED</promise> and explain which criteria are missing - do NOT mark the task as complete. \
   c. Only if ALL Verification Criteria are satisfied: \
      - Mark the main task checkbox as complete [x] \
      - Check off all Verification Criteria checkboxes for that task [x] \
2. Remove .ralph/PLAN.md. \
3. Update .ralph/PROGRESS.md with any learnings. \
4. Update @CLAUDE.md with any new features or changes. Ensure to use CLAUDE.md best practices and conventions: high‑level project context, clear guardrails, key commands, and links to deeper docs, while avoiding long prose and unnecessary detail. \
5. Update @README.md with any applicable changes. Update README.md only if: 1) New features were added that users should know about, 2) Setup/installation steps changed, 3) Configuration options were added/removed. \
If no README updates are needed, skip that step - do not ask.`

const BuiltInAgentsRefactorPrompt = `@CLAUDE.md \
I want you to refactor my CLAUDE.md file to follow progressive disclosure principles.

Follow these steps:

1. **Find contradictions**: Identify any instructions that conflict with each other. For each contradiction, ask me which version I want to keep.

2. **Identify the essentials**: Extract only what belongs in the root CLAUDE.md:
   - One-sentence project description
   - Package manager (if not npm)
   - Non-standard build/typecheck commands
   - Anything truly relevant to every single task

3. **Group the rest**: Organize remaining instructions into logical categories (e.g., TypeScript conventions, testing patterns, API design, Git workflow). For each group, create a separate markdown file.

4. **Create the file structure**: Output:
   - A minimal root CLAUDE.md with markdown links to the separate files
   - Each separate file with its relevant instructions
   - All sub md files MUST be in the docs/ folder

5. **Flag for deletion**: Identify any instructions that are:
   - Redundant (the agent already knows this)
   - Too vague to be actionable
   - Overly obvious (like "write clean code")`

const BuiltInSelfImprovementPrompt = `@.ralph/PRD.md @.ralph/PROGRESS.md \
Analyze the codebase for improvements, but ONLY add CRITICAL and HIGH priority issues as new tasks to .ralph/PRD.md. \
1. Review the entire codebase for: \
   - Code smells (duplication, complexity, poor naming, magic numbers, etc.) \
   - Architecture issues (tight coupling, missing abstractions, scalability concerns) \
   - Missing functionality (gaps between PRD requirements and implementation) \
   - Technical debt (quick fixes that need refactoring, outdated patterns) \
   - Security concerns (missing validations, potential vulnerabilities) \
   - Performance issues (N+1 queries, missing indexes, inefficient algorithms) \
2. STRICT FILTERING: Only document issues that are: \
   - CRITICAL: Security vulnerabilities, data loss risks, production outages \
   - HIGH: Severe performance issues (>2s latency, memory leaks, N+1 queries affecting >100 records), security gaps, data integrity issues \
   - Must have measurable, documented impact (e.g., '50+ API calls per request', 'potential DoS vulnerability', 'memory leak causing crashes after 10 minutes') \
3. EXPLICIT EXCLUSIONS - DO NOT add: \
   - Code smells that don't affect functionality (naming, minor duplication without maintenance burden) \
   - Missing functionality already tracked in .ralph/PRD.md \
   - Technical debt that doesn't block features or cause bugs \
   - Performance optimizations for code paths that aren't bottlenecks \
   - Low/Medium priority issues (unless they're security-related) \
4. DEDUPLICATION: Before adding any issue to .ralph/PRD.md: \
   - Read .ralph/PRD.md and check for duplicates (similar issues already documented as tasks) \
   - Only add if it provides new critical information or indicates the issue is more severe than previously documented \
   - If a similar issue exists, update the existing task rather than creating a duplicate \
5. PRD TASK FORMAT: For each finding, add a new task to the Tasks section of .ralph/PRD.md following this exact format: \
   - [ ] **Task [N]: [Issue Category] - [Brief Issue Description]** \
   \
   **Description:** [Clear description of the issue with measurable impact. Include: category (Security/Data Integrity/Production Blocker/Performance), location (file path and relevant code sections), specific evidence of impact, and suggested approach for addressing it] \
   \
   **Verification Criteria:** \
   - [ ] [Specific, measurable criterion 1 - e.g., 'Security vulnerability is patched and tested'] \
   - [ ] [Specific, measurable criterion 2 - e.g., 'Performance issue resolved with benchmark showing <500ms latency'] \
   - [ ] [Specific, measurable criterion 3 - e.g., 'All affected code paths are covered by tests'] \
   \
   **Complexity:** [easy/medium/hard - assess based on the scope of work needed to address the issue] \
   \
   --- \
6. Add new tasks to the end of the Tasks section in .ralph/PRD.md, maintaining the existing format and structure. \
7. If there are no CRITICAL or HIGH priority issues to add, output 'No critical issues found' and skip updating .ralph/PRD.md. \
Complete the analysis and update .ralph/PRD.md - do not ask for confirmation before adding items.`

const BuiltInCommitPrompt = `@.ralph/PRD.md @.ralph/PROGRESS.md \
Review the changes and commit with a clear message. \
Use format: 'feat: [brief description]' or 'fix: [brief description]' based on the changes. \
Review git status, stage all relevant changes, and commit - do not ask for approval. \
If there are no changes to commit, output 'No changes to commit' and proceed to next iteration.`

const BuiltInGuardrailVerifyPrompt = `@GUARDRAILS.md @.ralph/PRD.md @.ralph/PLAN.md @.ralph/PROGRESS.md @CLAUDE.md \
1. Read @GUARDRAILS.md and understand all guardrail rules (they verify PRD tasks, plans, and outcome compliance—not code style). \
2. Verify that the completed work and the way the PRD task and plan specified it comply with the guardrails. \
3. If any guardrail rule is violated (e.g. hardcoded secret, missing verification criterion, prod mocks): apply fixes and list what was fixed. Do not perform a general code-style or lint review. \
4. If fully compliant with all guardrails, output <promise>COMPLIANT</promise>. \
Do not ask for confirmation. Proceed immediately. \
If you are blocked, output <promise>BLOCKED</promise> and explain.`

const BuiltInPlanGuardrailVerifyPrompt = `@GUARDRAILS.md @.ralph/PLAN.md @.ralph/PRD.md @.ralph/PROGRESS.md \
1. Read @GUARDRAILS.md and understand all guardrail rules (they verify PRD tasks and plans, not code style). \
2. Review the plan in .ralph/PLAN.md (not the implementation). Determine if any planned steps would violate any guardrail. \
3. If violations exist: revise .ralph/PLAN.md to comply, then list what was fixed. \
4. If compliant (or after fixing), output <promise>COMPLIANT</promise>. \
5. If the plan cannot be made compliant without changing the PRD task, output <promise>BLOCKED</promise> and explain. \
Do not ask for confirmation. Proceed immediately.`

const BuiltInSamplePRD = `# Product Requirements Document

## Overview

This PRD outlines the requirements for [PROJECT NAME]. The goal is to [CLEAR DESCRIPTION OF WHAT THIS PRD IS TRYING TO ACCOMPLISH].

## Objectives

- [Primary objective 1]
- [Primary objective 2]
- [Primary objective 3]

## Tasks

- [ ] **Task 1: [Task Name]**

  **Description:** [Clear description of what needs to be done]

  **Verification Criteria:**
  - [ ] [Specific, measurable criterion 1]
  - [ ] [Specific, measurable criterion 2]
  - [ ] [Specific, measurable criterion 3]

  **Complexity:** [easy/medium/hard]

---

- [ ] **Task 2: [Task Name]**

  **Description:** [Clear description of what needs to be done]

  **Verification Criteria:**
  - [ ] [Specific, measurable criterion 1]
  - [ ] [Specific, measurable criterion 2]
  - [ ] [Specific, measurable criterion 3]

  **Complexity:** [easy/medium/hard]

---

- [ ] **Task 3: [Task Name]**

  **Description:** [Clear description of what needs to be done]

  **Verification Criteria:**
  - [ ] [Specific, measurable criterion 1]
  - [ ] [Specific, measurable criterion 2]
  - [ ] [Specific, measurable criterion 3]

  **Complexity:** [easy/medium/hard]

---

## Notes

- Add any additional context, constraints, or considerations here
- Update this section as needed during development
`

// BuiltInGuardrailsTemplate is the default content for GUARDRAILS.md when created via --init-guardrails.
const BuiltInGuardrailsTemplate = `# Guardrails

This file defines rules that the Ralph loop uses to verify PRD tasks and the plans that implement them (and that the resulting work complies). Guardrails are not for code-style or lint checks. When GUARDRAILS.md exists, Ralph verifies the plan before implementation and PRD/outcome compliance after implementation. Edit or remove rules to match your project.

## Requirements and tasks

- PRD tasks must have clear, measurable verification criteria.
- No task or plan may require hardcoded secrets, API keys, or passwords; use env vars or a secrets manager.
- No task or plan may require mocking or fake data in dev or prod code paths; mocks only in tests.
- Plans must not propose steps that violate project constraints (e.g. single function over 300 lines if that is a project rule).

## Security and constraints

- Tasks and plans must not propose hardcoded secrets, raw SQL string concatenation, or logging of credentials.
- Input validation and parameterized queries (or prepared statements) must be in scope where applicable.
- Do not log secrets, tokens, or full credentials (mask or omit).

## Testing

- New or changed behavior must have tests (unit and/or integration as appropriate); plans must include test coverage.
- No mocking of data in code paths that affect dev or prod; mocks only in tests.
- Tests must pass; fix or update tests when implementation changes.

## Documentation and maintenance

- Update README, CLAUDE.md, or docs when adding features, changing setup, or changing config.
- Do not leave dead code or commented-out blocks without a ticket or follow-up.
`

// Prompt file names in .ralph directory
const (
	SystemPromptFile             = ".ralph/system_prompt.txt"
	PlanningPromptFile           = ".ralph/planning_prompt.txt"
	ImplementationPromptFile     = ".ralph/implementation_prompt.txt"
	CleanupPromptFile           = ".ralph/cleanup_prompt.txt"
	GuardrailVerifyPromptFile    = ".ralph/guardrail_verify_prompt.txt"
	PlanGuardrailVerifyPromptFile = ".ralph/plan_guardrail_verify_prompt.txt"
	AgentsRefactorPromptFile     = ".ralph/agents_refactor_prompt.txt"
	SelfImprovementPromptFile    = ".ralph/self_improvement_prompt.txt"
	CommitPromptFile             = ".ralph/commit_prompt.txt"
	SamplePRDFile                = ".ralph/PRD.md"
)

// getSystemPrompt returns the system prompt, checking .ralph directory first, then falling back to built-in
func getSystemPrompt() (string, error) {
	content, err := readFileContent(SystemPromptFile)
	if err == nil {
		return content, nil
	}
	// Fall back to built-in prompt
	return BuiltInSystemPrompt, nil
}

// getStepPrompt returns the prompt for a given step, checking .ralph directory first, then falling back to built-in
func getStepPrompt(stepNum int) string {
	var filename string
	var builtInPrompt string

	switch stepNum {
	case 1:
		filename = PlanningPromptFile
		builtInPrompt = BuiltInPlanningPrompt
	case 2:
		filename = ImplementationPromptFile
		builtInPrompt = BuiltInImplementationPrompt
	case 3:
		filename = CleanupPromptFile
		builtInPrompt = BuiltInCleanupPrompt
	case 4:
		filename = AgentsRefactorPromptFile
		builtInPrompt = BuiltInAgentsRefactorPrompt
	case 5:
		filename = SelfImprovementPromptFile
		builtInPrompt = BuiltInSelfImprovementPrompt
	case 6:
		filename = CommitPromptFile
		builtInPrompt = BuiltInCommitPrompt
	default:
		return builtInPrompt
	}

	content, err := readFileContent(filename)
	if err == nil {
		return content
	}
	// Fall back to built-in prompt
	return builtInPrompt
}

// getGuardrailVerifyPrompt returns the guardrail verification prompt, checking .ralph directory first, then falling back to built-in.
func getGuardrailVerifyPrompt() string {
	content, err := readFileContent(GuardrailVerifyPromptFile)
	if err == nil {
		return content
	}
	return BuiltInGuardrailVerifyPrompt
}

// getPlanGuardrailVerifyPrompt returns the plan guardrail verification prompt, checking .ralph directory first, then falling back to built-in.
func getPlanGuardrailVerifyPrompt() string {
	content, err := readFileContent(PlanGuardrailVerifyPromptFile)
	if err == nil {
		return content
	}
	return BuiltInPlanGuardrailVerifyPrompt
}

// exportPrompts writes all built-in prompts to the .ralph directory
func exportPrompts() error {
	// Ensure .ralph directory exists
	if err := os.MkdirAll(".ralph", 0755); err != nil {
		return fmt.Errorf("failed to create .ralph directory: %v", err)
	}

	// Export system prompt
	if err := writeFileContent(SystemPromptFile, BuiltInSystemPrompt); err != nil {
		return fmt.Errorf("failed to write system prompt: %v", err)
	}

	// Export step prompts
	stepPrompts := map[string]string{
		PlanningPromptFile:           BuiltInPlanningPrompt,
		ImplementationPromptFile:     BuiltInImplementationPrompt,
		CleanupPromptFile:            BuiltInCleanupPrompt,
		GuardrailVerifyPromptFile:    BuiltInGuardrailVerifyPrompt,
		PlanGuardrailVerifyPromptFile: BuiltInPlanGuardrailVerifyPrompt,
		AgentsRefactorPromptFile:     BuiltInAgentsRefactorPrompt,
		SelfImprovementPromptFile:    BuiltInSelfImprovementPrompt,
		CommitPromptFile:             BuiltInCommitPrompt,
	}

	for filename, prompt := range stepPrompts {
		if err := writeFileContent(filename, prompt); err != nil {
			return fmt.Errorf("failed to write %s: %v", filename, err)
		}
	}

	// Export sample PRD (only if it doesn't exist)
	prdStatus := "(skipped - file already exists)"
	if _, err := os.Stat(SamplePRDFile); os.IsNotExist(err) {
		if err := writeFileContent(SamplePRDFile, BuiltInSamplePRD); err != nil {
			return fmt.Errorf("failed to write sample PRD: %v", err)
		}
		prdStatus = "(sample)"
	}

	fmt.Println("✅ Exported all prompts to .ralph directory:")
	fmt.Printf("   - %s\n", SystemPromptFile)
	for filename := range stepPrompts {
		fmt.Printf("   - %s\n", filename)
	}
	fmt.Printf("   - %s %s\n", SamplePRDFile, prdStatus)
	fmt.Println("\nYou can now customize these prompts by editing the files in .ralph/")
	if prdStatus == "(sample)" {
		fmt.Println("Edit .ralph/PRD.md to define your project requirements with tasks and verification criteria.")
	}

	return nil
}

// writeFileContent writes content to a file, creating parent directories if needed
func writeFileContent(filename string, content string) error {
	// Ensure parent directory exists
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(filename, []byte(content), 0644)
}

// initProject creates the minimum files needed to get started
// If description is provided, it will interactively create a PRD using Claude
func initProject(description string) error {
	// Ensure .ralph directory exists
	if err := os.MkdirAll(".ralph", 0755); err != nil {
		return fmt.Errorf("failed to create .ralph directory: %v", err)
	}

	// Clear out old files from previous runs
	oldFiles := []string{".ralph/PROGRESS.md", ".ralph/PLAN.md", "PROGRESS.md", "PLAN.md"}
	for _, file := range oldFiles {
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			// Log but don't fail if file can't be removed (e.g., permissions issue)
			fmt.Printf("⚠️  Warning: could not remove %s: %v\n", file, err)
		}
	}

	// If description is provided, use PRD creation flow
	if description != "" {
		return createPRD(description)
	}

	// Otherwise, use existing sample PRD behavior
	prdStatus := "(created)"
	if _, err := os.Stat(SamplePRDFile); err == nil {
		prdStatus = "(already exists - skipped)"
	} else {
		if err := writeFileContent(SamplePRDFile, BuiltInSamplePRD); err != nil {
			return fmt.Errorf("failed to write PRD: %v", err)
		}
	}

	fmt.Println("✅ Initialized Ralph project:")
	fmt.Printf("   - %s %s\n", SamplePRDFile, prdStatus)
	fmt.Println("\nNext steps:")
	if prdStatus == "(created)" {
		fmt.Println("1. Edit .ralph/PRD.md to define your project requirements with tasks and verification criteria.")
	} else {
		fmt.Println("1. Review .ralph/PRD.md to ensure it contains your project requirements.")
	}
	fmt.Println("2. Run: ./ralph <iterations> to start the development loop")
	fmt.Println("3. Optional: Run: ./ralph --export-prompts to customize prompts")

	return nil
}

// initGuardrails creates GUARDRAILS.md by having Claude analyze the project and generate tailored guardrails.
func initGuardrails() error {
	return createGuardrailsWithClaude()
}
