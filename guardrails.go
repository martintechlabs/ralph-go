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

GUARDRAILS.md is used by Ralph to verify PRD tasks and the plans that implement them (and that the resulting work complies). It is NOT for code-style or lint checks. Rules should be concrete constraints that tasks and plans must not violate (e.g. no tasks requiring hardcoded secrets, no prod mocks, clear verification criteria, security/constraint rules). Include specific, actionable rules—not placeholders.`

const GuardrailsCreationUserPrompt = `
Review the attached project files to understand this application (language, framework, conventions, testing approach, and security considerations).

Generate a complete GUARDRAILS.md file focused on verifying PRD tasks and plans. Use sections such as:
- **Requirements and tasks** – e.g. tasks must have clear verification criteria; no tasks may require hardcoded secrets, prod mocks, or bypassing env/config; constraints that plans must respect (e.g. no single function over 300 lines if that is a project rule)
- **Security and constraints** – what tasks and plans must not propose (e.g. no raw SQL in task scope, input validation required, no logging of secrets)
- **Testing** – e.g. no mocking of data in dev/prod code paths; mocks only in tests; coverage expectations as a constraint for what plans must include
- **Documentation and maintenance** – when PRD/docs must be updated (e.g. when adding features or changing setup)

De-emphasize pure code-style (e.g. "run gofmt"). Frame rules as constraints that PRD tasks and implementation plans must not violate. Adapt to the project: Go may mention parameterized queries, env for config; Python may mention no bare except, type hints where they affect contracts; etc.

OUTPUT REQUIREMENTS:
- Start your response directly with "# Guardrails" (with a short intro paragraph explaining that guardrails verify PRD tasks and plans, not code style).
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
	fmt.Println("Edit GUARDRAILS.md to refine rules. When present, Ralph verifies the plan and PRD/outcome compliance against it (before and after each implementation step).")
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
