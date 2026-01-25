package main

import (
	"fmt"
	"os"
	"strings"
)

const PRDCreationSystemPrompt = `You are a supportive product manager creating a comprehensive PRD for the Ralph Wiggum autonomous development loop.

AUTONOMOUS MODE: You are operating in fully autonomous mode.

CRITICAL RULES:
- DO NOT ask follow-up questions
- DO NOT request clarification
- DO NOT ask for confirmation before proceeding
- Make reasonable assumptions about missing details based on best practices
- If information is missing, infer from context and create a reasonable PRD
- Complete the PRD fully without asking if you should continue

Your goal is to create a complete Product Requirements Document based on the user's description. You should:
1. Analyze the project description provided
2. Make reasonable assumptions about missing details based on best practices
3. Generate a comprehensive PRD with well-defined tasks, verification criteria, and complexity assessments

Be thorough and create a production-ready PRD that covers all aspects of the project.`

const PRDCreationUserPromptTemplate = `The user wants to build: %s

CRITICAL: Create a comprehensive Product Requirements Document (PRD) based on this description. DO NOT ask questions - make reasonable assumptions and proceed immediately.

Analyze the project and make reasonable assumptions about:

1. **Project Overview** - What problem is being solved?
2. **Target Audience** - Who is the primary user?
3. **Core Features** - What are the 3-5 core features in order of priority?
4. **Tech Stack** - Recommend appropriate technologies (frontend, backend, database, etc.)
5. **Architecture** - Suggest appropriate architecture (monolithic, microservices, serverless, etc.)
6. **UI/UX** - Suggest design approach and requirements
7. **Data & State Management** - What data needs to be managed?
8. **Authentication & Security** - What auth is needed?
9. **Third-Party Integrations** - What external services might be needed?
10. **Development Constraints** - Consider common constraints
11. **Success Criteria** - How to measure completion?

Generate a complete PRD in the EXACT format specified below. Break down the project into atomic, verifiable tasks with clear complexity assessments.

## PRD Format Requirements

The PRD MUST start with:
# Product Requirements Document

## Overview
[Brief description of what you're building and why]

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

[Continue with more tasks as needed]

## Notes
- Add any additional context, constraints, or considerations here
- Update this section as needed during development

CRITICAL FORMATTING RULES:
- Tasks must use "- [ ]" checkboxes (not completed)
- Each task must have a bold name like "**Task 1: [Name]**"
- Description must be on its own line with "**Description:**" prefix
- Verification Criteria must be a bullet list with checkboxes
- Complexity must be on its own line with "**Complexity:**" prefix
- Tasks must be separated by "---" horizontal rules
- Use proper markdown formatting throughout

Generate tasks based on the features and requirements gathered. Tasks should be:
- **Atomic**: Each task should be completable in one iteration
- **Verifiable**: Each task should have clear success criteria
- **Ordered**: Tasks should be in logical dependency order
- **Complexity**: Assess each task as easy, medium, or hard

Once you have all the information, generate the complete PRD in the format above.

CRITICAL OUTPUT REQUIREMENTS:
- Output ONLY the PRD markdown content - nothing else
- Do NOT include any file path messages like "The PRD is now saved at..."
- Do NOT include any explanatory text before or after the PRD
- Do NOT include any status messages or confirmations
- Start your response directly with "# Product Requirements Document"
- End your response with the closing of the PRD (after the Notes section)
- The entire output should be the PRD markdown and nothing else`

// createPRD orchestrates the PRD creation process
func createPRD(description string) error {
	// Check if PRD already exists
	if _, err := os.Stat(SamplePRDFile); err == nil {
		fmt.Printf("⚠️  Warning: %s already exists\n", SamplePRDFile)
		fmt.Println("The existing PRD will be overwritten with the new one.")
		fmt.Println()
	}

	fmt.Println("🚀 Starting PRD creation...")
	fmt.Printf("📝 Project description: %s\n", description)
	fmt.Println()
	fmt.Println("Claude is analyzing your project and generating a comprehensive PRD...")
	fmt.Println()

	// Run the discovery flow
	prdContent, err := prdDiscoveryFlow(description)
	if err != nil {
		return fmt.Errorf("PRD discovery flow failed: %v", err)
	}

	// Write the PRD to file
	if err := writeFileContent(SamplePRDFile, prdContent); err != nil {
		return fmt.Errorf("failed to write PRD: %v", err)
	}

	fmt.Println()
	fmt.Println("✅ PRD created successfully!")
	fmt.Printf("   - %s\n", SamplePRDFile)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("1. Review .ralph/PRD.md to ensure it meets your requirements")
	fmt.Println("2. Run: ./ralph <iterations> to start the development loop")
	fmt.Println("3. Optional: Run: ./ralph --export-prompts to customize prompts")

	return nil
}

// prdDiscoveryFlow runs the interactive discovery conversation with Claude
func prdDiscoveryFlow(description string) (string, error) {
	systemPrompt := PRDCreationSystemPrompt
	userPrompt := fmt.Sprintf(PRDCreationUserPromptTemplate, description)

	// Run Claude with the discovery prompt
	result, err := runClaude(TimeoutPRDCreation, systemPrompt, userPrompt)
	if err != nil {
		// Error is already formatted by formatClaudeError(), just wrap it
		return "", fmt.Errorf("PRD creation failed: %w", err)
	}

	if !result.Success {
		return "", fmt.Errorf("PRD creation failed: %s", result.Output)
	}

	// Extract PRD content from the output
	prdContent := extractPRDFromOutput(result.Output)
	if prdContent == "" {
		// Check if Claude asked questions instead of creating a PRD
		outputLower := strings.ToLower(result.Output)
		if strings.Contains(outputLower, "could you please") ||
			strings.Contains(outputLower, "please provide") ||
			strings.Contains(outputLower, "what kind of") ||
			strings.Contains(outputLower, "need more") ||
			strings.Contains(outputLower, "more details") ||
			strings.Contains(outputLower, "more information") {
			return "", fmt.Errorf("PRD creation failed: Claude asked questions instead of creating a PRD. Please ensure the description is more detailed, or the PRD creation prompt enforces autonomous mode.")
		}
		return "", fmt.Errorf("failed to extract PRD from Claude output. Output length: %d characters", len(result.Output))
	}

	return prdContent, nil
}

// extractPRDFromOutput extracts the PRD markdown from Claude's output
func extractPRDFromOutput(output string) string {
	// Remove any file path messages that Claude might have included
	// Look for patterns like "The PRD is now saved at..." or "saved at..."
	lines := strings.Split(output, "\n")
	var filteredLines []string
	inPRD := false
	
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Skip lines that look like file path messages
		if strings.Contains(strings.ToLower(trimmed), "saved at") ||
			strings.Contains(strings.ToLower(trimmed), "the prd is now") ||
			strings.Contains(strings.ToLower(trimmed), "prd saved") ||
			(strings.HasPrefix(trimmed, "/") && strings.Contains(trimmed, "PRD.md")) {
			continue
		}
		
		// Start capturing when we find the PRD header
		if strings.Contains(trimmed, "# Product Requirements Document") {
			inPRD = true
			filteredLines = append(filteredLines, line)
			continue
		}
		
		// If we're in the PRD, continue capturing
		if inPRD {
			// Stop if we hit JSON metadata or other non-PRD content
			if strings.Contains(trimmed, `"session_id"`) ||
				strings.Contains(trimmed, `"total_cost_usd"`) ||
				strings.Contains(trimmed, `"usage"`) ||
				strings.Contains(trimmed, `"modelUsage"`) ||
				strings.Contains(trimmed, `"permission_denials"`) ||
				strings.Contains(trimmed, `"uuid"`) {
				break
			}
			
			// Stop if we hit another obvious non-PRD message
			if strings.Contains(strings.ToLower(trimmed), "ready for development") ||
				strings.Contains(strings.ToLower(trimmed), "next steps") ||
				(strings.HasPrefix(trimmed, "✅") && i > 10) {
				break
			}
			
			filteredLines = append(filteredLines, line)
		}
	}
	
	// If we found the PRD header and captured content, return it
	if inPRD && len(filteredLines) > 0 {
		result := strings.TrimSpace(strings.Join(filteredLines, "\n"))
		if len(result) > 100 { // Ensure we got substantial content
			return result
		}
	}
	
	// Fallback: Try to find markdown code block with ```markdown
	markdownStart := strings.Index(output, "```markdown")
	if markdownStart != -1 {
		// Find the end of the code block
		contentStart := markdownStart + len("```markdown")
		// Skip any leading whitespace/newlines
		contentStart = skipWhitespace(output, contentStart)
		
		// Find the closing ```
		markdownEnd := strings.Index(output[contentStart:], "```")
		if markdownEnd != -1 {
			return strings.TrimSpace(output[contentStart : contentStart+markdownEnd])
		}
	}

	// Try to find regular code block with ```
	codeStart := strings.Index(output, "```")
	if codeStart != -1 {
		contentStart := codeStart + len("```")
		// Skip any whitespace
		contentStart = skipWhitespace(output, contentStart)
		// Skip potential language identifier (like "markdown", "text", etc.)
		for contentStart < len(output) && output[contentStart] != '\n' {
			contentStart++
		}
		contentStart = skipWhitespace(output, contentStart)
		
		// Find the closing ```
		codeEnd := strings.Index(output[contentStart:], "```")
		if codeEnd != -1 {
			return strings.TrimSpace(output[contentStart : contentStart+codeEnd])
		}
	}

	// If no code block found, look for the PRD structure directly
	// Check if output contains "# Product Requirements Document"
	prdHeader := "# Product Requirements Document"
	headerIndex := strings.Index(output, prdHeader)
	if headerIndex != -1 {
		// Extract from header to end, but filter out JSON metadata and file path messages
		remaining := output[headerIndex:]
		
		// First, try to find where JSON metadata starts (if it appears after the PRD)
		// Look for common JSON metadata patterns
		jsonPatterns := []string{
			`"session_id"`,
			`"total_cost_usd"`,
			`"usage"`,
			`"modelUsage"`,
			`"permission_denials"`,
			`"uuid"`,
		}
		
		// Find the earliest occurrence of any JSON pattern
		earliestJSON := len(remaining)
		for _, pattern := range jsonPatterns {
			if idx := strings.Index(remaining, pattern); idx != -1 && idx < earliestJSON {
				earliestJSON = idx
			}
		}
		
		// Also look for file path messages
		filePathPatterns := []string{
			"The PRD is now saved at",
			"saved at",
			"ready for development",
		}
		
		earliestFilePath := len(remaining)
		for _, pattern := range filePathPatterns {
			if idx := strings.Index(remaining, pattern); idx != -1 && idx < earliestFilePath {
				earliestFilePath = idx
			}
		}
		
		// Use the earliest stopping point
		earliestStop := earliestJSON
		if earliestFilePath < earliestStop {
			earliestStop = earliestFilePath
		}
		
		// If we found a stopping point, extract only up to that point
		if earliestStop < len(remaining) {
			// Look backwards from the stop point to find a good break point (like end of a line or section)
			// Find the last newline before the stop point
			lastNewline := strings.LastIndex(remaining[:earliestStop], "\n")
			if lastNewline != -1 {
				remaining = remaining[:lastNewline]
			} else {
				remaining = remaining[:earliestStop]
			}
		}
		
		// Clean up the result
		result := strings.TrimSpace(remaining)
		
		// Remove any trailing JSON-like or file path content that might have slipped through
		lines := strings.Split(result, "\n")
		var cleanLines []string
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			// Skip lines that look like JSON metadata or file paths
			if strings.Contains(trimmed, `"session_id"`) ||
				strings.Contains(trimmed, `"total_cost_usd"`) ||
				strings.Contains(trimmed, `"usage"`) ||
				strings.Contains(trimmed, `"modelUsage"`) ||
				strings.Contains(trimmed, `"permission_denials"`) ||
				strings.Contains(trimmed, `"uuid"`) ||
				strings.Contains(strings.ToLower(trimmed), "saved at") ||
				strings.Contains(strings.ToLower(trimmed), "the prd is now") ||
				(strings.HasPrefix(trimmed, "/") && strings.Contains(trimmed, "PRD.md")) {
				break
			}
			cleanLines = append(cleanLines, line)
		}
		
		if len(cleanLines) > 0 {
			return strings.TrimSpace(strings.Join(cleanLines, "\n"))
		}
		
		return result
	}

	// Last resort: return the entire output if it looks like markdown
	if strings.Contains(output, "## Overview") || strings.Contains(output, "## Tasks") {
		return strings.TrimSpace(output)
	}

	return ""
}

// skipWhitespace skips whitespace characters starting from the given index
func skipWhitespace(s string, start int) int {
	for start < len(s) && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	return start
}
