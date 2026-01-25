package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// LinearConfig represents the Linear configuration from TOML file
type LinearConfig struct {
	Token        string `toml:"token"`
	Project      string `toml:"project"`       // Project ID to filter tickets
	EscalateUser string `toml:"escalate_user"`
	BaseBranch   string `toml:"base_branch"`  // Base branch to create feature branches from (defaults to "main" or "master")
}

// ManagerState represents the resume state for manager mode
type ManagerState struct {
	IssueID    string
	BranchName string
	Iteration  int
}

// LinearClient handles Linear API interactions
type LinearClient struct {
	Token   string
	BaseURL string
}

// LinearIssue represents a Linear issue/ticket
type LinearIssue struct {
	ID          string
	Identifier  string
	Title       string
	Description string
	Priority    float64
	Estimate    *float64
	State       struct {
		Name string
		ID   string
	}
	Team struct {
		ID   string
		Name string
		Key  string
	}
	Assignee *struct {
		ID          string
		Name        string
		DisplayName string
		Email       string
	}
	Project *struct {
		ID   string
		Name string
	}
	Labels struct {
		Nodes []struct {
			ID   string
			Name string
		}
	}
	CreatedAt   string
	UpdatedAt   string
	DueDate     *string
	StartedAt   *string
	CompletedAt *string
	URL         string
}

// LinearUser represents a Linear user
type LinearUser struct {
	ID          string
	Name        string
	DisplayName string
	Email       string
}

// GraphQLResponse represents a generic GraphQL response
type GraphQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// loadLinearConfig loads and parses the Linear config TOML file
func loadLinearConfig(filename string) (*LinearConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var config LinearConfig
	if err := toml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %v", err)
	}

	if config.Token == "" {
		return nil, fmt.Errorf("token is required in config file")
	}
	if config.Project == "" {
		return nil, fmt.Errorf("project is required in config file")
	}
	if config.EscalateUser == "" {
		return nil, fmt.Errorf("escalate_user is required in config file")
	}

	return &config, nil
}

// NewLinearClient creates a new Linear API client
func NewLinearClient(token string) *LinearClient {
	return &LinearClient{
		Token:   token,
		BaseURL: LinearAPIEndpoint,
	}
}

// executeGraphQL executes a GraphQL query/mutation
func (c *LinearClient) executeGraphQL(query string, variables map[string]interface{}) (json.RawMessage, error) {
	payload := map[string]interface{}{
		"query": query,
	}
	if variables != nil {
		payload["variables"] = variables
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	req, err := http.NewRequest("POST", c.BaseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	// Linear API uses the API key directly in Authorization header
	req.Header.Set("Authorization", c.Token)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	var graphqlResp GraphQLResponse
	if err := json.Unmarshal(body, &graphqlResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	if len(graphqlResp.Errors) > 0 {
		var errorMsgs []string
		for _, e := range graphqlResp.Errors {
			errorMsgs = append(errorMsgs, e.Message)
		}
		return nil, fmt.Errorf("GraphQL errors: %s", strings.Join(errorMsgs, "; "))
	}

	return graphqlResp.Data, nil
}

// fetchTodoTickets fetches tickets in "Todo" state, ordered by priority
// Filters by projectID (must be project UUID, not slug)
func (c *LinearClient) fetchTodoTickets(projectID string) ([]LinearIssue, error) {
	query := `
		query($projectId: ID!) {
			issues(
				filter: {
					state: { name: { eq: "Todo" } }
					project: { id: { eq: $projectId } }
				}
			) {
				nodes {
					id
					identifier
					title
					description
					priority
					estimate
					state {
						name
						id
					}
					team {
						id
						name
						key
					}
					assignee {
						id
						name
						displayName
						email
					}
					project {
						id
						name
					}
					labels {
						nodes {
							id
							name
						}
					}
					createdAt
					updatedAt
					dueDate
					startedAt
					completedAt
					url
				}
			}
		}
	`

	variables := map[string]interface{}{
		"projectId": projectID,
	}

	data, err := c.executeGraphQL(query, variables)
	if err != nil {
		return nil, err
	}

	var result struct {
		Issues struct {
			Nodes []LinearIssue `json:"nodes"`
		} `json:"issues"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse issues: %v", err)
	}

	// Sort by priority (lower number = higher priority)
	issues := result.Issues.Nodes
	for i := 0; i < len(issues)-1; i++ {
		for j := i + 1; j < len(issues); j++ {
			if issues[i].Priority > issues[j].Priority {
				issues[i], issues[j] = issues[j], issues[i]
			}
		}
	}

	return issues, nil
}

// getIssueStateID gets the state ID for a given state name
func (c *LinearClient) getIssueStateID(teamID, stateName string) (string, error) {
	query := `
		query($teamId: ID!, $stateName: String!) {
			workflowStates(filter: { team: { id: { eq: $teamId } }, name: { eq: $stateName } }) {
				nodes {
					id
					name
				}
			}
		}
	`

	variables := map[string]interface{}{
		"teamId":    teamID,
		"stateName": stateName,
	}

	data, err := c.executeGraphQL(query, variables)
	if err != nil {
		return "", err
	}

	var result struct {
		WorkflowStates struct {
			Nodes []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"nodes"`
		} `json:"workflowStates"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("failed to parse workflow states: %v", err)
	}

	if len(result.WorkflowStates.Nodes) == 0 {
		return "", fmt.Errorf("state '%s' not found for team", stateName)
	}

	return result.WorkflowStates.Nodes[0].ID, nil
}

// updateTicketStatus updates a ticket's state
func (c *LinearClient) updateTicketStatus(issueID, teamID, stateName string) error {
	stateID, err := c.getIssueStateID(teamID, stateName)
	if err != nil {
		return fmt.Errorf("failed to get state ID: %v", err)
	}

	mutation := `
		mutation($issueId: String!, $stateId: String!) {
			issueUpdate(id: $issueId, input: { stateId: $stateId }) {
				success
			}
		}
	`

	variables := map[string]interface{}{
		"issueId": issueID,
		"stateId": stateID,
	}

	_, err = c.executeGraphQL(mutation, variables)
	return err
}

// findUserByUsername finds a user by their display name (username)
func (c *LinearClient) findUserByUsername(username string) (*LinearUser, error) {
	query := `
		query($username: String!) {
			users(filter: { displayName: { eq: $username } }) {
				nodes {
					id
					name
					displayName
					email
				}
			}
		}
	`

	variables := map[string]interface{}{
		"username": username,
	}

	data, err := c.executeGraphQL(query, variables)
	if err != nil {
		return nil, err
	}

	var result struct {
		Users struct {
			Nodes []LinearUser `json:"nodes"`
		} `json:"users"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse users: %v", err)
	}

	if len(result.Users.Nodes) == 0 {
		return nil, fmt.Errorf("user '%s' not found", username)
	}

	return &result.Users.Nodes[0], nil
}

// getWorkspaceInfo gets workspace information including URL slug
func (c *LinearClient) getWorkspaceInfo() (string, error) {
	query := `
		query {
			organization {
				urlKey
			}
		}
	`

	data, err := c.executeGraphQL(query, nil)
	if err != nil {
		return "", err
	}

	var result struct {
		Organization struct {
			URLKey string `json:"urlKey"`
		} `json:"organization"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("failed to parse organization: %v", err)
	}

	return result.Organization.URLKey, nil
}

// addTicketComment adds a comment to a ticket and optionally tags users
// Note: Linear uses profile URLs for mentions: https://linear.app/{workspace}/profiles/{username}
func (c *LinearClient) addTicketComment(issueID, comment string, usernames []string) error {
	commentBody := comment
	if len(usernames) > 0 {
		// Get workspace URL key for constructing profile URLs
		workspaceKey, err := c.getWorkspaceInfo()
		if err != nil {
			// If we can't get workspace, just use @mentions as fallback
			fmt.Printf("⚠️  Warning: Could not get workspace info for mentions: %v\n", err)
			mentions := []string{}
			for _, username := range usernames {
				mentions = append(mentions, "@"+username)
			}
			if len(mentions) > 0 {
				commentBody = strings.Join(mentions, " ") + "\n\n" + comment
			}
		} else {
			// Use profile URLs for mentions (using username, not UUID)
			mentions := []string{}
			for _, username := range usernames {
				profileURL := fmt.Sprintf("https://linear.app/%s/profiles/%s", workspaceKey, username)
				mentions = append(mentions, profileURL)
			}
			if len(mentions) > 0 {
				commentBody = strings.Join(mentions, " ") + "\n\n" + comment
			}
		}
	}

	mutation := `
		mutation($issueId: String!, $body: String!) {
			commentCreate(input: { issueId: $issueId, body: $body }) {
				success
				comment {
					id
				}
			}
		}
	`

	variables := map[string]interface{}{
		"issueId": issueID,
		"body":    commentBody,
	}

	_, err := c.executeGraphQL(mutation, variables)
	return err
}

// slugify converts a string to a URL-friendly slug
func slugify(s string) string {
	// Convert to lowercase
	s = strings.ToLower(s)
	// Replace spaces with hyphens
	s = strings.ReplaceAll(s, " ", "-")
	// Remove special characters, keep only alphanumeric and hyphens
	reg := regexp.MustCompile("[^a-z0-9-]+")
	s = reg.ReplaceAllString(s, "")
	// Remove multiple consecutive hyphens
	reg = regexp.MustCompile("-+")
	s = reg.ReplaceAllString(s, "-")
	// Remove leading/trailing hyphens
	s = strings.Trim(s, "-")
	return s
}

// createGitBranch creates and checks out a new git branch
// baseBranch is the branch to checkout before creating the new branch (defaults to "main" or "master" if empty)
func createGitBranch(branchName string, baseBranch string) error {
	// Determine base branch to use
	if baseBranch == "" {
		// Try to detect default branch (main or master)
		// Check if main exists
		checkMain := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/main")
		if err := checkMain.Run(); err == nil {
			baseBranch = "main"
		} else {
			// Check if master exists
			checkMaster := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/master")
			if err := checkMaster.Run(); err == nil {
				baseBranch = "master"
			} else {
				return fmt.Errorf("failed to determine base branch (tried 'main' and 'master'): neither branch exists")
			}
		}
	}

	// Ensure we're on the base branch first
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	currentBranch, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %v", err)
	}

	branchStr := strings.TrimSpace(string(currentBranch))
	if branchStr != baseBranch {
		// Try to checkout the base branch
		checkoutBase := exec.Command("git", "checkout", baseBranch)
		if err := checkoutBase.Run(); err != nil {
			return fmt.Errorf("not on %s and failed to checkout: %v", baseBranch, err)
		}
	}

	// Create and checkout new branch
	cmd = exec.Command("git", "checkout", "-b", branchName)
	if err := cmd.Run(); err != nil {
		// Branch might already exist, try to checkout
		cmd = exec.Command("git", "checkout", branchName)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to create/checkout branch: %v", err)
		}
	}

	return nil
}

// validateGitSetup validates that git remote is configured and GitHub CLI is available
func validateGitSetup() error {
	// Check if git remote is configured
	cmd := exec.Command("git", "remote", "-v")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check git remotes: %v", err)
	}

	remoteOutput := strings.TrimSpace(string(output))
	if remoteOutput == "" {
		return fmt.Errorf("no git remote configured. Please add a remote with: git remote add origin <url>")
	}

	// Check if remote URL is GitHub (github.com)
	lines := strings.Split(remoteOutput, "\n")
	hasGitHubRemote := false
	for _, line := range lines {
		if strings.Contains(line, "github.com") {
			hasGitHubRemote = true
			break
		}
	}

	if !hasGitHubRemote {
		return fmt.Errorf("git remote does not appear to be GitHub. PR creation requires GitHub")
	}

	// Check if GitHub CLI is installed
	ghCmd := exec.Command("gh", "--version")
	if err := ghCmd.Run(); err != nil {
		return fmt.Errorf("GitHub CLI (gh) is not installed. Please install it from https://cli.github.com/")
	}

	// Check if GitHub CLI is authenticated
	authCmd := exec.Command("gh", "auth", "status")
	authOutput, err := authCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("GitHub CLI is not authenticated. Please run: gh auth login\nOutput: %s", string(authOutput))
	}

	// Verify authentication is valid (check for "Logged in" in output)
	if !strings.Contains(string(authOutput), "Logged in") {
		return fmt.Errorf("GitHub CLI authentication appears invalid. Please run: gh auth login")
	}

	// Check if .ralph directory is in .gitignore
	gitignorePath := ".gitignore"
	gitignoreContent, err := os.ReadFile(gitignorePath)
	if err != nil {
		// .gitignore doesn't exist, create it with .ralph/
		gitignoreContent = []byte(".ralph/\n")
		if err := os.WriteFile(gitignorePath, gitignoreContent, 0644); err != nil {
			return fmt.Errorf("failed to create .gitignore: %v", err)
		}
		fmt.Printf("ℹ️  Created .gitignore with .ralph/ entry\n")
	} else {
		// Check if .ralph is already in .gitignore
		content := string(gitignoreContent)
		hasRalphIgnore := false
		scanner := bufio.NewScanner(strings.NewReader(content))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			// Check for .ralph or .ralph/ (with or without leading ./)
			if line == ".ralph" || line == ".ralph/" || line == "./.ralph" || line == "./.ralph/" {
				hasRalphIgnore = true
				break
			}
		}
		if !hasRalphIgnore {
			// Append .ralph/ to .gitignore
			file, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				return fmt.Errorf("failed to open .gitignore for writing: %v", err)
			}
			defer file.Close()
			
			// Add newline if file doesn't end with one
			contentBytes := gitignoreContent
			if len(contentBytes) > 0 && contentBytes[len(contentBytes)-1] != '\n' {
				if _, err := file.WriteString("\n"); err != nil {
					return fmt.Errorf("failed to write to .gitignore: %v", err)
				}
			}
			
			// Add .ralph/ entry
			if _, err := file.WriteString(".ralph/\n"); err != nil {
				return fmt.Errorf("failed to write .ralph/ to .gitignore: %v", err)
			}
			fmt.Printf("ℹ️  Added .ralph/ to .gitignore\n")
		}
	}

	return nil
}

// pushBranchToRemote pushes a branch to the remote repository
func pushBranchToRemote(branchName string) error {
	// Check if branch is already pushed
	cmd := exec.Command("git", "ls-remote", "--heads", "origin", branchName)
	output, err := cmd.Output()
	if err == nil && strings.TrimSpace(string(output)) != "" {
		// Branch already exists on remote, try to push anyway (might need to update)
		fmt.Printf("ℹ️  Branch %s already exists on remote, pushing updates...\n", branchName)
	}

	// Push branch to remote with upstream tracking
	cmd = exec.Command("git", "push", "-u", "origin", branchName)
	output, err = cmd.CombinedOutput()
	if err != nil {
		// Check if error is because branch is already up to date
		outputStr := string(output)
		if strings.Contains(outputStr, "Everything up-to-date") {
			fmt.Printf("ℹ️  Branch %s is already up to date on remote\n", branchName)
			return nil
		}
		return fmt.Errorf("failed to push branch to remote: %v\nOutput: %s", err, outputStr)
	}

	return nil
}

// createPullRequest creates a pull request using GitHub CLI
func createPullRequest(branchName, baseBranch, issueIdentifier, issueTitle, issueURL, issueDescription string) (string, error) {
	// Push branch first
	if err := pushBranchToRemote(branchName); err != nil {
		return "", fmt.Errorf("failed to push branch: %v", err)
	}

	// Build PR title
	prTitle := issueTitle
	if issueIdentifier != "" {
		prTitle = fmt.Sprintf("%s: %s", issueIdentifier, issueTitle)
	}

	// Build PR body
	var bodyParts []string
	bodyParts = append(bodyParts, fmt.Sprintf("Closes Linear ticket: %s", issueURL))
	if issueDescription != "" {
		// Truncate description if too long (GitHub PR body limit is ~65KB, but keep it reasonable)
		desc := issueDescription
		if len(desc) > 5000 {
			desc = desc[:5000] + "\n\n... (description truncated)"
		}
		bodyParts = append(bodyParts, "\n## Description")
		bodyParts = append(bodyParts, desc)
	}
	bodyParts = append(bodyParts, fmt.Sprintf("\n## Branch\n`%s`", branchName))
	bodyParts = append(bodyParts, "\n---\n*This PR was automatically created by Ralph*")

	prBody := strings.Join(bodyParts, "\n")

	// Create PR using GitHub CLI
	cmd := exec.Command("gh", "pr", "create",
		"--title", prTitle,
		"--body", prBody,
		"--base", baseBranch,
		"--head", branchName,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := string(output)
		// Check if PR already exists
		if strings.Contains(outputStr, "already exists") || strings.Contains(outputStr, "pull request already exists") {
			// Try to get the existing PR URL
			getPRCmd := exec.Command("gh", "pr", "view", branchName, "--json", "url", "--jq", ".url")
			prOutput, prErr := getPRCmd.Output()
			if prErr == nil {
				prURL := strings.TrimSpace(string(prOutput))
				if prURL != "" {
					fmt.Printf("ℹ️  Pull request already exists: %s\n", prURL)
					return prURL, nil
				}
			}
			return "", fmt.Errorf("pull request already exists for branch %s", branchName)
		}
		return "", fmt.Errorf("failed to create pull request: %v\nOutput: %s", err, outputStr)
	}

	// Extract PR URL from output
	outputStr := strings.TrimSpace(string(output))
	// GitHub CLI typically outputs the PR URL
	if strings.HasPrefix(outputStr, "http") {
		return outputStr, nil
	}

	// If URL not in output, try to get it
	getPRCmd := exec.Command("gh", "pr", "view", branchName, "--json", "url", "--jq", ".url")
	prOutput, err := getPRCmd.Output()
	if err == nil {
		prURL := strings.TrimSpace(string(prOutput))
		if prURL != "" {
			return prURL, nil
		}
	}

	// Fallback: return a message indicating PR was created
	return "https://github.com/<repo>/pull/<number> (created but URL not retrieved)", nil
}

// saveManagerState saves the manager state to file
func saveManagerState(state *ManagerState) error {
	dir := filepath.Dir(ManagerStateFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	file, err := os.Create(ManagerStateFile)
	if err != nil {
		return err
	}
	defer file.Close()

	fmt.Fprintf(file, "issue_id=%s\n", state.IssueID)
	fmt.Fprintf(file, "branch_name=%s\n", state.BranchName)
	fmt.Fprintf(file, "iteration=%d\n", state.Iteration)

	return nil
}

// loadManagerState loads the manager state from file
func loadManagerState() (*ManagerState, error) {
	if _, err := os.Stat(ManagerStateFile); os.IsNotExist(err) {
		return nil, nil
	}

	file, err := os.Open(ManagerStateFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	state := &ManagerState{}
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
		case "issue_id":
			state.IssueID = value
		case "branch_name":
			state.BranchName = value
		case "iteration":
			fmt.Sscanf(value, "%d", &state.Iteration)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return state, nil
}

// clearManagerState removes the manager state file
func clearManagerState() error {
	if _, err := os.Stat(ManagerStateFile); os.IsNotExist(err) {
		return nil
	}
	return os.Remove(ManagerStateFile)
}

// getCurrentGitBranch gets the current git branch name
func getCurrentGitBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %v", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// extractIssueIDFromBranch extracts Linear issue ID from branch name
// Branch format: linear/{issue-id}-{slug}
// Returns issue ID if pattern matches, empty string otherwise
func extractIssueIDFromBranch(branchName string) string {
	// Pattern: linear/{issue-id}-{slug}
	// Linear issue IDs are UUIDs (format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx)
	pattern := regexp.MustCompile(`^linear/([a-f0-9-]+)-`)
	matches := pattern.FindStringSubmatch(branchName)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

// detectBranchBasedRecovery detects recovery from current branch
// Checks if we're on a branch that matches a Linear ticket pattern
// and if that ticket is "In Progress"
func detectBranchBasedRecovery(client *LinearClient) (*ManagerState, error) {
	// Get current branch
	branchName, err := getCurrentGitBranch()
	if err != nil {
		// Not a git repo or other error - silently continue
		return nil, nil
	}

	// Check if branch matches Linear pattern
	issueID := extractIssueIDFromBranch(branchName)
	if issueID == "" {
		// Branch doesn't match pattern - not a Linear branch
		return nil, nil
	}

	// Verify ticket exists and is "In Progress"
	valid, err := client.verifyIssueState(issueID, "In Progress")
	if err != nil {
		// Error checking ticket - log warning but don't fail
		fmt.Printf("⚠️  Warning: Could not verify ticket state for branch %s: %v\n", branchName, err)
		return nil, nil
	}

	if !valid {
		// Ticket not in "In Progress" - not a recoverable state
		return nil, nil
	}

	// Valid recovery state found
	return &ManagerState{
		IssueID:    issueID,
		BranchName: branchName,
		Iteration:  1, // Start from iteration 1, the ralph loop will handle resume from its own state
	}, nil
}

// verifyIssueState verifies that an issue exists and is in the expected state
func (c *LinearClient) verifyIssueState(issueID, expectedState string) (bool, error) {
	query := `
		query($issueId: String!) {
			issue(id: $issueId) {
				id
				state {
					name
				}
			}
		}
	`

	variables := map[string]interface{}{
		"issueId": issueID,
	}

	data, err := c.executeGraphQL(query, variables)
	if err != nil {
		return false, err
	}

	var result struct {
		Issue struct {
			ID    string `json:"id"`
			State struct {
				Name string `json:"name"`
			} `json:"state"`
		} `json:"issue"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return false, fmt.Errorf("failed to parse issue: %v", err)
	}

	if result.Issue.ID == "" {
		return false, nil // Issue doesn't exist
	}

	return result.Issue.State.Name == expectedState, nil
}

// IterationProgress contains information about what was accomplished in an iteration
type IterationProgress struct {
	Iteration      int
	MaxIterations  int
	StepsCompleted []string
	CommitMessage  string
	FilesChanged   []string
}

// ProgressCallback is called after each iteration completes
type ProgressCallback func(progress IterationProgress) error

// getLastCommitMessage gets the last git commit message
func getLastCommitMessage() string {
	cmd := exec.Command("git", "log", "-1", "--pretty=%B")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// getChangedFiles gets list of files changed in the last commit
func getChangedFiles() []string {
	cmd := exec.Command("git", "diff", "--name-only", "HEAD~1", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		// If there's no previous commit, check unstaged changes
		cmd = exec.Command("git", "diff", "--name-only")
		output, err = cmd.Output()
		if err != nil {
			return []string{}
		}
	}
	
	files := strings.Split(strings.TrimSpace(string(output)), "\n")
	var result []string
	for _, file := range files {
		if file != "" {
			result = append(result, file)
		}
	}
	return result
}

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

// runRalphLoop runs the main ralph loop with the given iterations
// Returns true if PRD was completed, false if iteration limit reached, error on failure
// progressCallback is called after each iteration completes (optional)
func runRalphLoop(iterations int, progressCallback ProgressCallback) (bool, error) {
	// Verify required files exist
	for _, filename := range RequiredFiles {
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			return false, fmt.Errorf("required file %s not found", filename)
		}
	}

	// Resume detection (non-interactive for manager mode)
	startIteration := 1
	resumeStep := 0
	resumeState, resumeStepNum, err := detectResumeWithPrompt(iterations, false)
	if err != nil {
		return false, fmt.Errorf("error detecting resume state: %v", err)
	}

	if resumeState != nil {
		startIteration = resumeState.Iteration
		resumeStep = resumeStepNum
	}

	// Main loop - similar to main.go but returns instead of exiting
	for i := startIteration; i <= iterations; i++ {
		// Save state at iteration start
		state := &State{
			Iteration:         i,
			MaxIterations:     iterations,
			CurrentStep:       1,
			LastCompletedStep: 0,
		}
		if err := saveState(state); err != nil {
			return false, fmt.Errorf("error saving state: %v", err)
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
				return false, fmt.Errorf("error saving state: %v", err)
			}

			result, err := step1Planning(i, iterations)
			if err != nil {
				return false, fmt.Errorf("error in Step 1: %v", err)
			}

			if result.Complete {
				clearState()
				return true, nil // PRD complete
			}

			if result.Blocked {
				return false, fmt.Errorf("blocked during planning")
			}

			state.CurrentStep = 2
			state.LastCompletedStep = 1
			if err := saveState(state); err != nil {
				return false, fmt.Errorf("error saving state: %v", err)
			}
		}

		// Step 2: Implementation
		if skipToStep <= 2 {
			state.CurrentStep = 2
			state.LastCompletedStep = 1
			if err := saveState(state); err != nil {
				return false, fmt.Errorf("error saving state: %v", err)
			}

			result, err := step2Implementation(i, iterations)
			if err != nil {
				return false, fmt.Errorf("error in Step 2: %v", err)
			}

			if result.Blocked {
				return false, fmt.Errorf("blocked during implementation")
			}

			state.CurrentStep = 3
			state.LastCompletedStep = 2
			if err := saveState(state); err != nil {
				return false, fmt.Errorf("error saving state: %v", err)
			}
		}

		// Step 3: Cleanup
		if skipToStep <= 3 {
			state.CurrentStep = 3
			state.LastCompletedStep = 2
			if err := saveState(state); err != nil {
				return false, fmt.Errorf("error saving state: %v", err)
			}

			_, err := step3Cleanup(i, iterations)
			if err != nil {
				return false, fmt.Errorf("error in Step 3: %v", err)
			}

			state.CurrentStep = 4
			state.LastCompletedStep = 3
			if err := saveState(state); err != nil {
				return false, fmt.Errorf("error saving state: %v", err)
			}
		}

		// Step 4: CLAUDE.md Refactoring
		if skipToStep <= 4 {
			state.CurrentStep = 4
			state.LastCompletedStep = 3
			if err := saveState(state); err != nil {
				return false, fmt.Errorf("error saving state: %v", err)
			}

			_, err := step4AgentsRefactor(i, iterations)
			if err != nil {
				return false, fmt.Errorf("error in Step 4: %v", err)
			}

			state.CurrentStep = 5
			state.LastCompletedStep = 4
			if err := saveState(state); err != nil {
				return false, fmt.Errorf("error saving state: %v", err)
			}
		}

		// Step 5: Self-Improvement (every 5th iteration)
		if i%5 == 0 {
			if skipToStep <= 5 {
				state.CurrentStep = 5
				state.LastCompletedStep = 4
				if err := saveState(state); err != nil {
					return false, fmt.Errorf("error saving state: %v", err)
				}

				_, err := step5SelfImprovement(i, iterations)
				if err != nil {
					return false, fmt.Errorf("error in Step 5: %v", err)
				}

				state.CurrentStep = 6
				state.LastCompletedStep = 5
				if err := saveState(state); err != nil {
					return false, fmt.Errorf("error saving state: %v", err)
				}
			}
		}

		// Step 6: Commit
		if skipToStep <= 6 || skipToStep == 0 {
			state.CurrentStep = 6
			state.LastCompletedStep = 5
			if err := saveState(state); err != nil {
				return false, fmt.Errorf("error saving state: %v", err)
			}

			_, err := step6Commit(i, iterations)
			if err != nil {
				return false, fmt.Errorf("error in Step 6: %v", err)
			}

			state.CurrentStep = 0
			state.LastCompletedStep = 6
			if err := saveState(state); err != nil {
				return false, fmt.Errorf("error saving state: %v", err)
			}
		}

		// Gather progress information and call callback
		if progressCallback != nil {
			progress := IterationProgress{
				Iteration:     i,
				MaxIterations: iterations,
			}

			// Determine which steps were completed (based on LastCompletedStep)
			var stepsCompleted []string
			if state.LastCompletedStep >= 1 {
				stepsCompleted = append(stepsCompleted, "Planning")
			}
			if state.LastCompletedStep >= 2 {
				stepsCompleted = append(stepsCompleted, "Implementation")
			}
			if state.LastCompletedStep >= 3 {
				stepsCompleted = append(stepsCompleted, "Cleanup")
			}
			if state.LastCompletedStep >= 4 {
				stepsCompleted = append(stepsCompleted, "CLAUDE.md Refactoring")
			}
			if state.LastCompletedStep >= 5 {
				stepsCompleted = append(stepsCompleted, "Self-Improvement")
			}
			if state.LastCompletedStep >= 6 {
				stepsCompleted = append(stepsCompleted, "Commit")
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

		// Clear resume step after first iteration
		if i == startIteration {
			resumeStep = 0
		}
	}

	// Iteration limit reached
	clearState()
	return false, nil
}

// listTeams lists all teams in the workspace (helper to find team_id)
func (c *LinearClient) listTeams() ([]struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Key  string `json:"key"`
}, error) {
	query := `
		query {
			teams {
				nodes {
					id
					name
					key
				}
			}
		}
	`

	data, err := c.executeGraphQL(query, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Teams struct {
			Nodes []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
				Key  string `json:"key"`
			} `json:"nodes"`
		} `json:"teams"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse teams: %v", err)
	}

	return result.Teams.Nodes, nil
}

// listProjects lists all projects in the workspace (helper to find project UUID)
func (c *LinearClient) listProjects() ([]struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	SlugID string `json:"slugId"`
}, error) {
	query := `
		query {
			projects {
				nodes {
					id
					name
					slugId
				}
			}
		}
	`

	data, err := c.executeGraphQL(query, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Projects struct {
			Nodes []struct {
				ID     string `json:"id"`
				Name   string `json:"name"`
				SlugID string `json:"slugId"`
			} `json:"nodes"`
		} `json:"projects"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse projects: %v", err)
	}

	return result.Projects.Nodes, nil
}

// listPendingTickets lists all pending tickets from Linear (for testing connectivity)
func listPendingTickets(configFile string) error {
	// Load Linear config
	config, err := loadLinearConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %v", err)
	}

	// Initialize Linear client
	client := NewLinearClient(config.Token)

	// First, try to list projects to help find the correct UUID if needed
	fmt.Println("ℹ️  Listing available projects to help find the correct project ID...")
	projects, err := client.listProjects()
	if err != nil {
		fmt.Printf("⚠️  Warning: Could not list projects: %v\n", err)
	} else if len(projects) > 0 {
		fmt.Println("\nAvailable projects:")
		for _, project := range projects {
			fmt.Printf("  - %s (Slug: %s, ID: %s)\n", project.Name, project.SlugID, project.ID)
		}
		fmt.Println()
	} else {
		fmt.Println("⚠️  No projects found in workspace.")
		fmt.Println()
	}

	// Fetch all tickets by project (temporarily showing all, not just Todo)
	query := `
		query($projectId: ID!) {
			issues(
				filter: {
					project: { id: { eq: $projectId } }
				}
			) {
				nodes {
					id
					identifier
					title
					description
					priority
					estimate
					state {
						name
						id
					}
					team {
						id
						name
						key
					}
					assignee {
						id
						name
						displayName
						email
					}
					project {
						id
						name
					}
					labels {
						nodes {
							id
							name
						}
					}
					createdAt
					updatedAt
					dueDate
					startedAt
					completedAt
					url
				}
			}
		}
	`

	variables := map[string]interface{}{
		"projectId": config.Project,
	}

	data, err := client.executeGraphQL(query, variables)
	if err != nil {
		errorMsg := fmt.Sprintf("failed to fetch tickets (filtering by project: %s): %v", config.Project, err)
		if len(projects) > 0 {
			errorMsg += "\n\nTip: Use the project ID (UUID) from the list above, not the slug."
		} else {
			errorMsg += "\n\nTip: The project ID must be a UUID, not a slug. Check your Linear workspace for the project UUID."
		}
		return fmt.Errorf(errorMsg)
	}

	var result struct {
		Issues struct {
			Nodes []LinearIssue `json:"nodes"`
		} `json:"issues"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("failed to parse issues: %v", err)
	}

	tickets := result.Issues.Nodes

	// Sort by priority (lower number = higher priority)
	for i := 0; i < len(tickets)-1; i++ {
		for j := i + 1; j < len(tickets); j++ {
			if tickets[i].Priority > tickets[j].Priority {
				tickets[i], tickets[j] = tickets[j], tickets[i]
			}
		}
	}

	if len(tickets) == 0 {
		fmt.Println("✅ No tickets found in this project.")
		return nil
	}

	fmt.Printf("📋 Found %d ticket(s) in project (all states):\n\n", len(tickets))
	for i, ticket := range tickets {
		priorityName := "No priority"
		switch int(ticket.Priority) {
		case 1:
			priorityName = "Urgent"
		case 2:
			priorityName = "High"
		case 3:
			priorityName = "Medium"
		case 4:
			priorityName = "Low"
		}

		fmt.Printf("%d. %s\n", i+1, ticket.Title)
		fmt.Printf("   Identifier: %s\n", ticket.Identifier)
		fmt.Printf("   ID: %s\n", ticket.ID)
		fmt.Printf("   URL: %s\n", ticket.URL)
		fmt.Printf("   State: %s\n", ticket.State.Name)
		fmt.Printf("   Priority: %s (%.0f)\n", priorityName, ticket.Priority)
		
		if ticket.Estimate != nil {
			fmt.Printf("   Estimate: %.0f\n", *ticket.Estimate)
		}
		
		if ticket.Assignee != nil {
			fmt.Printf("   Assignee: %s (%s)\n", ticket.Assignee.DisplayName, ticket.Assignee.Email)
		} else {
			fmt.Printf("   Assignee: Unassigned\n")
		}
		
		fmt.Printf("   Team: %s (%s)\n", ticket.Team.Name, ticket.Team.Key)
		
		if ticket.Project != nil {
			fmt.Printf("   Project: %s\n", ticket.Project.Name)
		}
		
		if len(ticket.Labels.Nodes) > 0 {
			labelNames := make([]string, len(ticket.Labels.Nodes))
			for j, label := range ticket.Labels.Nodes {
				labelNames[j] = label.Name
			}
			fmt.Printf("   Labels: %s\n", strings.Join(labelNames, ", "))
		}
		
		fmt.Printf("   Created: %s\n", ticket.CreatedAt)
		fmt.Printf("   Updated: %s\n", ticket.UpdatedAt)
		
		if ticket.DueDate != nil {
			fmt.Printf("   Due Date: %s\n", *ticket.DueDate)
		}
		
		if ticket.StartedAt != nil {
			fmt.Printf("   Started: %s\n", *ticket.StartedAt)
		}
		
		if ticket.CompletedAt != nil {
			fmt.Printf("   Completed: %s\n", *ticket.CompletedAt)
		}
		
		if ticket.Description != "" {
			// Truncate description if too long
			desc := ticket.Description
			if len(desc) > 200 {
				desc = desc[:200] + "..."
			}
			fmt.Printf("   Description: %s\n", desc)
		}
		fmt.Println()
	}

	return nil
}

// runManagerMode is the main manager loop
func runManagerMode(configFile string, iterations int) error {
	// Load Linear config
	config, err := loadLinearConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %v", err)
	}

	// Validate git setup (remote and GitHub CLI)
	if err := validateGitSetup(); err != nil {
		return fmt.Errorf("git setup validation failed: %v", err)
	}

	// Initialize Linear client
	client := NewLinearClient(config.Token)

	// Check for resume state
	managerState, err := loadManagerState()
	if err != nil {
		return fmt.Errorf("failed to load manager state: %v", err)
	}

	if managerState != nil && managerState.IssueID != "" {
		// Verify ticket still exists and is in "In Progress"
		valid, err := client.verifyIssueState(managerState.IssueID, "In Progress")
		if err != nil {
			fmt.Printf("⚠️  Error verifying resume state: %v\n", err)
			clearManagerState()
			managerState = nil
		} else if !valid {
			fmt.Printf("⚠️  Resume state invalid (ticket not in 'In Progress'), starting fresh\n")
			clearManagerState()
			managerState = nil
		} else {
			// Resume from existing ticket
			fmt.Printf("🔄 Resuming from ticket %s on branch %s\n", managerState.IssueID, managerState.BranchName)
			// Checkout the branch
			if err := createGitBranch(managerState.BranchName, config.BaseBranch); err != nil {
				fmt.Printf("⚠️  Failed to checkout branch %s: %v\n", managerState.BranchName, err)
				clearManagerState()
				managerState = nil
			}
		}
	}

	// If no saved state or saved state is invalid, check current branch for recovery
	if managerState == nil || managerState.IssueID == "" {
		branchState, err := detectBranchBasedRecovery(client)
		if err != nil {
			// Log error but don't fail - continue to normal flow
			fmt.Printf("⚠️  Warning: Error detecting branch-based recovery: %v\n", err)
		} else if branchState != nil {
			// Found valid recovery state from branch
			fmt.Printf("🔄 Detected in-progress ticket from branch %s, resuming\n", branchState.BranchName)
			managerState = branchState
			// Save the state so it persists
			if err := saveManagerState(managerState); err != nil {
				fmt.Printf("⚠️  Warning: Failed to save manager state: %v\n", err)
			}
			// Ensure we're on the branch (we should already be, but verify)
			if err := createGitBranch(managerState.BranchName, config.BaseBranch); err != nil {
				fmt.Printf("⚠️  Failed to checkout branch %s: %v\n", managerState.BranchName, err)
				managerState = nil
			}
		}
	}

	// Main loop
	for {
		var issue *LinearIssue
		var branchName string

		if managerState != nil && managerState.IssueID != "" {
			// Resuming - fetch the issue
			query := `
				query($issueId: String!) {
					issue(id: $issueId) {
						id
						title
						description
						priority
						state {
							name
							id
						}
						team {
							id
						}
					}
				}
			`

			variables := map[string]interface{}{
				"issueId": managerState.IssueID,
			}

			data, err := client.executeGraphQL(query, variables)
			if err != nil {
				return fmt.Errorf("failed to fetch resume issue: %v", err)
			}

			var result struct {
				Issue LinearIssue `json:"issue"`
			}

			if err := json.Unmarshal(data, &result); err != nil {
				return fmt.Errorf("failed to parse issue: %v", err)
			}

			if result.Issue.ID == "" {
				fmt.Printf("⚠️  Resume issue not found, starting fresh\n")
				clearManagerState()
				managerState = nil
				continue
			}

			issue = &result.Issue
			branchName = managerState.BranchName
		} else {
			// Fetch Todo tickets
			tickets, err := client.fetchTodoTickets(config.Project)
			if err != nil {
				return fmt.Errorf("failed to fetch tickets: %v", err)
			}

			if len(tickets) == 0 {
				fmt.Println("ℹ️  No Todo tickets found. Sleeping for 1 minute and checking again...")
				time.Sleep(1 * time.Minute)
				continue
			}

			// Select highest priority ticket (first one, already sorted)
			issue = &tickets[0]
			fmt.Printf("📋 Selected ticket: %s (Priority: %.0f)\n", issue.Title, issue.Priority)

			// Create git branch
			issueSlug := slugify(issue.Title)
			branchName = fmt.Sprintf("linear/%s-%s", issue.ID, issueSlug)
			if err := createGitBranch(branchName, config.BaseBranch); err != nil {
				return fmt.Errorf("failed to create git branch: %v", err)
			}

			// Create PRD from ticket first (so we can include it in the comment)
			prdDescription := fmt.Sprintf("%s\n\n%s", issue.Title, issue.Description)
			if err := createPRD(prdDescription); err != nil {
				// Error creating PRD - escalate
				errorComment := fmt.Sprintf("❌ Error creating PRD for ticket:\n\n**Error:** %v\n**Branch:** `%s`", err, branchName)
				usernames := []string{config.EscalateUser}
				if err := client.addTicketComment(issue.ID, errorComment, usernames); err != nil {
					fmt.Printf("⚠️  Warning: failed to add error comment: %v\n", err)
				}

				clearManagerState()
				return fmt.Errorf("failed to create PRD: %v", err)
			}

			// Read PRD content to include in comment
			prdContent := ""
			if prdFile, err := os.ReadFile(".ralph/PRD.md"); err == nil {
				prdContent = string(prdFile)
			}

			// Add comment to ticket with branch info and PRD, tagging escalate_user
			var commentParts []string
			commentParts = append(commentParts, fmt.Sprintf("Starting work on branch: `%s`", branchName))
			if prdContent != "" {
				commentParts = append(commentParts, "\n\n**PRD:**")
				commentParts = append(commentParts, "```markdown")
				commentParts = append(commentParts, prdContent)
				commentParts = append(commentParts, "```")
			}
			comment := strings.Join(commentParts, "\n")
			usernames := []string{config.EscalateUser}
			if err := client.addTicketComment(issue.ID, comment, usernames); err != nil {
				fmt.Printf("⚠️  Warning: failed to add comment to ticket: %v\n", err)
			}

			// Save manager state
			managerState = &ManagerState{
				IssueID:    issue.ID,
				BranchName: branchName,
				Iteration:  1,
			}
			if err := saveManagerState(managerState); err != nil {
				return fmt.Errorf("failed to save manager state: %v", err)
			}

			// Update ticket to "In Progress"
			if err := client.updateTicketStatus(issue.ID, issue.Team.ID, "In Progress"); err != nil {
				return fmt.Errorf("failed to update ticket status: %v", err)
			}
		}

		// Create progress callback for Linear updates
		progressCallback := func(progress IterationProgress) error {
			var commentParts []string
			commentParts = append(commentParts, fmt.Sprintf("**Iteration %d/%d completed**", progress.Iteration, progress.MaxIterations))

			if len(progress.StepsCompleted) > 0 {
				commentParts = append(commentParts, "\n**Steps completed:**")
				for _, step := range progress.StepsCompleted {
					commentParts = append(commentParts, fmt.Sprintf("- ✅ %s", step))
				}
			}

			if progress.CommitMessage != "" {
				commentParts = append(commentParts, fmt.Sprintf("\n**Commit:** `%s`", progress.CommitMessage))
			}

			if len(progress.FilesChanged) > 0 {
				commentParts = append(commentParts, fmt.Sprintf("\n**Files changed:** %d", len(progress.FilesChanged)))
				if len(progress.FilesChanged) <= 10 {
					// Show all files if 10 or fewer
					for _, file := range progress.FilesChanged {
						commentParts = append(commentParts, fmt.Sprintf("- `%s`", file))
					}
				} else {
					// Show first 10 files if more than 10
					for _, file := range progress.FilesChanged[:10] {
						commentParts = append(commentParts, fmt.Sprintf("- `%s`", file))
					}
					commentParts = append(commentParts, fmt.Sprintf("- ... and %d more", len(progress.FilesChanged)-10))
				}
			}

			comment := strings.Join(commentParts, "\n")
			return client.addTicketComment(issue.ID, comment, nil)
		}

		// Run ralph loop
		completed, err := runRalphLoop(iterations, progressCallback)
		if err != nil {
			// Error during ralph execution - escalate
			errorComment := fmt.Sprintf("❌ Error during ralph execution:\n\n**Error:** %v\n**Branch:** `%s`", err, branchName)
			usernames := []string{config.EscalateUser}
			if err := client.addTicketComment(issue.ID, errorComment, usernames); err != nil {
				fmt.Printf("⚠️  Warning: failed to add error comment: %v\n", err)
			}

			// Update ticket back to "Todo"
			if err := client.updateTicketStatus(issue.ID, issue.Team.ID, "Todo"); err != nil {
				fmt.Printf("⚠️  Warning: failed to update ticket status: %v\n", err)
			}

			clearManagerState()
			return fmt.Errorf("ralph execution failed: %v", err)
		}

		if !completed {
			// Iteration limit reached - escalate
			errorComment := fmt.Sprintf("⚠️  Iteration limit (%d) reached but PRD not complete.\n\n**Branch:** `%s`\n\nPlease review and continue manually.", iterations, branchName)
			usernames := []string{config.EscalateUser}
			if err := client.addTicketComment(issue.ID, errorComment, usernames); err != nil {
				fmt.Printf("⚠️  Warning: failed to add error comment: %v\n", err)
			}

			// Update ticket back to "Todo"
			if err := client.updateTicketStatus(issue.ID, issue.Team.ID, "Todo"); err != nil {
				fmt.Printf("⚠️  Warning: failed to update ticket status: %v\n", err)
			}

			clearManagerState()
			return fmt.Errorf("iteration limit reached without completion")
		}

		// Success! Create pull request
		baseBranch := config.BaseBranch
		if baseBranch == "" {
			// Try to detect default branch (main or master)
			checkMain := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/main")
			if err := checkMain.Run(); err == nil {
				baseBranch = "main"
			} else {
				checkMaster := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/master")
				if err := checkMaster.Run(); err == nil {
					baseBranch = "master"
				} else {
					baseBranch = "main" // Default fallback
				}
			}
		}

		prURL, err := createPullRequest(branchName, baseBranch, issue.Identifier, issue.Title, issue.URL, issue.Description)
		if err != nil {
			// PR creation failed - escalate but don't fail the workflow
			errorComment := fmt.Sprintf("⚠️  Work completed but failed to create pull request:\n\n**Error:** %v\n**Branch:** `%s`", err, branchName)
			usernames := []string{config.EscalateUser}
			if err := client.addTicketComment(issue.ID, errorComment, usernames); err != nil {
				fmt.Printf("⚠️  Warning: failed to add error comment: %v\n", err)
			}
			fmt.Printf("⚠️  Warning: Failed to create pull request: %v\n", err)
		} else {
			fmt.Printf("✅ Pull request created: %s\n", prURL)
		}

		// Update ticket to "Done"
		var successCommentParts []string
		successCommentParts = append(successCommentParts, fmt.Sprintf("✅ Work completed successfully on branch: `%s`", branchName))
		if prURL != "" {
			successCommentParts = append(successCommentParts, fmt.Sprintf("\n**Pull Request:** %s", prURL))
		}
		successComment := strings.Join(successCommentParts, "\n")
		if err := client.addTicketComment(issue.ID, successComment, nil); err != nil {
			fmt.Printf("⚠️  Warning: failed to add success comment: %v\n", err)
		}

		if err := client.updateTicketStatus(issue.ID, issue.Team.ID, "Done"); err != nil {
			return fmt.Errorf("failed to update ticket to Done: %v", err)
		}

		fmt.Printf("✅ Ticket %s completed successfully!\n", issue.Title)

		// Clear manager state and continue to next ticket
		clearManagerState()
		managerState = nil
	}
}
