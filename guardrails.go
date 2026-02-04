package main

import (
	"fmt"
	"os"
	"strings"
)

const GuardrailsCreationSystemPrompt = `You are helping create a GUARDRAILS.md file for a project that uses the Ralph autonomous development loop.

AUTONOMOUS MODE: You are operating in fully autonomous mode.

CRITICAL RULES:
- DO NOT ask follow-up questions
- DO NOT request clarification
- DO NOT ask for confirmation
- Analyze the attached project files and infer language, framework, conventions, and risks
- Generate a complete GUARDRAILS.md tailored to this project
- Output ONLY the raw GUARDRAILS.md content—no explanation before or after

GUARDRAILS.md is used by Ralph to verify each implementation step. Rules should be concrete and project-appropriate (code style, security, testing, documentation/maintenance). Include specific, actionable rules—not placeholders.`

const GuardrailsCreationUserPrompt = `
Review the attached project files to understand this application (language, framework, conventions, testing approach, and security considerations).

Generate a complete GUARDRAILS.md file with sections such as:
- **Code style** – formatting, naming, no magic numbers, simplicity, file/function size
- **Security** – no hardcoded secrets, input validation, logging, database access
- **Testing** – coverage expectations, mocks only in tests, no disabled tests
- **Documentation and maintenance** – when to update README/docs, no dead code, imports at top

Adapt the rules to this project. For example: Go projects (go.mod) may mention go vet, gofmt, table-driven tests; Python may mention type hints, pytest, no bare except; Node may mention ESLint, Jest.

OUTPUT REQUIREMENTS:
- Start your response directly with "# Guardrails" (with a short intro paragraph explaining the file).
- End with the last line of the document. No trailing explanation.
- Output ONLY the markdown content—nothing else.`

// createGuardrailsWithClaude analyzes the project and generates GUARDRAILS.md using Claude.
func createGuardrailsWithClaude() error {
	if _, err := os.Stat(GuardrailsFile); err == nil {
		fmt.Printf("%s already exists\n", GuardrailsFile)
		return nil
	}

	// Gather project context: include @refs only for files that exist
	refs := gatherProjectRefs()
	if len(refs) == 0 {
		return fmt.Errorf("no project files found (README.md, CLAUDE.md, go.mod, package.json, etc.); add at least one so Claude can analyze the project")
	}

	prompt := strings.Join(refs, " ") + GuardrailsCreationUserPrompt

	fmt.Println("Analyzing project and generating GUARDRAILS.md...")
	fmt.Println()

	result, err := runClaude(TimeoutPRDCreation, GuardrailsCreationSystemPrompt, prompt)
	if err != nil {
		return fmt.Errorf("guardrails creation failed: %w", err)
	}
	if !result.Success {
		return fmt.Errorf("guardrails creation failed: %s", result.Output)
	}

	content := extractGuardrailsFromOutput(result.Output)
	if content == "" {
		return fmt.Errorf("could not extract GUARDRAILS.md from Claude output (length: %d)", len(result.Output))
	}

	if err := writeFileContent(GuardrailsFile, content); err != nil {
		return fmt.Errorf("failed to write %s: %w", GuardrailsFile, err)
	}

	fmt.Printf("✅ Created %s\n", GuardrailsFile)
	fmt.Println("Edit GUARDRAILS.md to refine rules. When present, Ralph verifies implementations against it after each implementation step.")
	return nil
}

// gatherProjectRefs returns @-prefixed paths for project files that exist (for Claude prompt).
func gatherProjectRefs() []string {
	candidates := []string{
		"README.md", "CLAUDE.md", "go.mod", "go.sum",
		"package.json", "requirements.txt", "pyproject.toml", "Cargo.toml",
	}
	var refs []string
	for _, name := range candidates {
		if _, err := os.Stat(name); err == nil {
			refs = append(refs, "@"+name)
		}
	}
	return refs
}

// extractGuardrailsFromOutput extracts GUARDRAILS.md markdown from Claude's output.
func extractGuardrailsFromOutput(output string) string {
	lines := strings.Split(output, "\n")
	var captured []string
	inDoc := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "# Guardrails") {
			inDoc = true
			captured = append(captured, line)
			continue
		}
		if inDoc {
			if strings.HasPrefix(trimmed, "```") ||
				strings.Contains(trimmed, `"session_id"`) ||
				strings.Contains(trimmed, `"total_cost_usd"`) {
				break
			}
			captured = append(captured, line)
		}
	}

	if inDoc && len(captured) > 0 {
		result := strings.TrimSpace(strings.Join(captured, "\n"))
		if len(result) > 100 {
			return result
		}
	}

	// Fallback: markdown code block
	if i := strings.Index(output, "```markdown"); i != -1 {
		start := i + len("```markdown")
		for start < len(output) && (output[start] == '\r' || output[start] == '\n') {
			start++
		}
		end := strings.Index(output[start:], "```")
		if end != -1 {
			return strings.TrimSpace(output[start : start+end])
		}
	}
	if i := strings.Index(output, "```"); i != -1 {
		start := i + len("```")
		for start < len(output) && output[start] != '\n' {
			start++
		}
		start++
		end := strings.Index(output[start:], "```")
		if end != -1 {
			return strings.TrimSpace(output[start : start+end])
		}
	}

	return ""
}
