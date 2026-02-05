# ralph-go

An autonomous development loop executor (Ralph Loop) that uses Claude AI to automatically plan, implement, test, and commit code changes based on a Product Requirements Document (PRD).

## What is Ralph?

Ralph is an automated development assistant that runs a structured workflow to complete tasks from a PRD. It operates autonomously, making decisions and implementing code without requiring constant human intervention. The tool executes a 6-step loop:

1. **Planning** - Reviews the PRD, selects tasks, and creates detailed implementation plans
2. **Implementation** - Writes code, runs tests, fixes errors, and ensures test coverage
3. **Cleanup** - Updates documentation, removes temporary files, and maintains project state
4. **CLAUDE.md Refactoring** - Refactors CLAUDE.md to follow progressive disclosure principles
5. **Self-Improvement** (every 5th iteration) - Analyzes the codebase for critical issues and technical debt
6. **Commit** - Commits changes with appropriate commit messages

Ralph can resume from checkpoints if interrupted, handles timeouts with retries, and automatically detects when work is complete or blocked.

### Two Modes of Operation

Ralph supports two modes:

- **Standalone Mode**: Works with a local PRD file (`.ralph/PRD.md`). Perfect for one-off tasks, personal projects, or when you want full control over the task list.

- **Manager Mode**: Integrates with Linear to automatically process tickets. Fetches tickets, creates branches, runs the development loop, and creates pull requests automatically. Ideal for teams using Linear for project management.

## Features

- **Autonomous operation** - Makes decisions independently without asking for confirmation
- **State persistence** - Automatically resumes from the last checkpoint if interrupted
- **Configurable prompts** - Customize behavior by overriding prompts in `.ralph` directory
- **Built-in defaults** - Works out of the box with sensible defaults
- **Single executable** - Easy deployment, no complex setup required
- **Error handling** - Automatic retries on timeouts, graceful handling of blockers
- **Linear Integration** - Automatically process tickets from Linear with manager mode
- **Progress tracking** - Real-time updates to Linear tickets as work progresses
- **Automatic PR creation** - Creates pull requests automatically when tickets are completed (manager mode)

## Prerequisites

- Go 1.21 or later (for building from source)
- The `claude` CLI tool installed and configured (Ralph uses this to interact with Claude AI)
- A `.ralph/PRD.md` file in your project directory (Product Requirements Document with tasks to complete)
  - Not required for manager mode (PRD is created from Linear ticket)
- For Linear manager mode:
  - Linear API token and project UUID
  - GitHub CLI (`gh`) installed and authenticated (for automatic PR creation)
  - Git remote configured (must be a GitHub repository)

You can create the PRD manually or use `./ralph --init [description]` to generate one automatically.

## Configuration

The `ralph` executable includes a standard set of prompts and configuration files built-in. To customize behavior, create a `.ralph` directory and place your configuration files there. Files in the `.ralph` directory will override the built-in defaults.

### Required Files

- `.ralph/PRD.md` - Product Requirements Document with tasks (checkboxes for incomplete tasks)

### Optional Configuration Files

You can export and customize the built-in prompts:

```bash
./ralph --export-prompts
```

This creates the following files in `.ralph/`:
- `system_prompt.txt` - Overall behavior and decision-making rules
- `planning_prompt.txt` - Planning prompt
- `implementation_prompt.txt` - Implementation prompt
- `cleanup_prompt.txt` - Cleanup prompt
- `agents_refactor_prompt.txt` - Agents refactor (CLAUDE.md) prompt
- `self_improvement_prompt.txt` - Self-improvement prompt
- `commit_prompt.txt` - Commit prompt
- `guardrail_verify_prompt.txt` - Guardrail verification prompt (used when GUARDRAILS.md exists)
- `plan_guardrail_verify_prompt.txt` - Plan guardrail verification prompt (used when GUARDRAILS.md exists)

If a `.ralph` directory doesn't exist or specific files are missing, the executable will use its built-in defaults.

**GUARDRAILS.md** (optional, project root): Guardrails verify that PRD tasks and plans (and the resulting work) comply with project rules—they are not for code-style or lint checks. When present, Ralph (1) verifies the **plan** against guardrails after planning and before implementation, and (2) verifies **PRD/plan/outcome compliance** after implementation and before cleanup/commit. Use `./ralph --init-guardrails` to create a template.

## Usage

Ralph has two modes: **Standalone Mode** (works with local PRD files) and **Manager Mode** (automatically processes Linear tickets). Choose the mode that fits your workflow.

### Standalone Mode

Use standalone mode when you have a PRD file and want to work on tasks locally:

```bash
# Run Ralph with a specified number of iterations
./ralph <iterations>

# Example: Run for 10 iterations
./ralph 10
```

### Initialize a New Project

```bash
# Create a basic PRD template
./ralph --init

# Create a PRD interactively with Claude based on a description
./ralph --init "Build a todo app with user authentication"
```

The `--init` command creates the minimum files needed to get started (`.ralph/PRD.md`). If you provide a description, Ralph will use Claude to generate a comprehensive PRD based on your project description, then simplify it so tasks are easy or medium and aimed at 15–20 minutes each.

### Export Prompts for Customization

```bash
# Export all built-in prompts to .ralph directory
./ralph --export-prompts
```

After exporting, you can edit the prompt files in `.ralph/` to customize Ralph's behavior.

### Initialize Guardrails

```bash
./ralph --init-guardrails
```

Analyzes the current project (README, CLAUDE.md, go.mod, package.json, etc.) and uses Claude to generate a tailored `GUARDRAILS.md` in the project root. The generated guardrails are PRD/plan-focused (constraints that tasks and plans must not violate, e.g. no hardcoded secrets, no prod mocks). If the file already exists, the command does nothing. When GUARDRAILS.md is present, Ralph verifies the plan before implementation and PRD/outcome compliance after implementation. Edit the generated file to refine rules.

### Simplify PRD

```bash
./ralph --simplify-prd
```

Reprocesses `.ralph/PRD.md` with the same simplification rules (easy/medium tasks, 15–20 min each). Completed tasks and their verification criteria are left unchanged; only incomplete tasks are simplified or split.

### Getting Help

```bash
./ralph --help
# or
./ralph -h
```

### Version Information

```bash
./ralph --version
# or
./ralph -v
```

### Manager Mode (Linear Integration)

Manager mode automatically processes tickets from Linear, running the development loop for each ticket. This mode is ideal for teams that use Linear for project management and want to automate the development workflow from ticket to pull request.

**When to use Manager Mode:**
- Your team uses Linear for project management
- You want to automate the full workflow: ticket → branch → code → PR
- You want progress updates automatically posted to Linear tickets
- You need automatic PR creation when tickets are completed

**When to use Standalone Mode:**
- You're working on a personal project or one-off task
- You prefer to manage tasks in a local PRD file
- You don't use Linear or want more control over the workflow

```bash
# List all tickets in a project (for testing connectivity)
./ralph --tickets <config-file>

# Run manager mode to automatically process tickets
./ralph --manager <config-file> <iterations>
```

**Manager Mode Features:**
- Automatically fetches tickets in "Todo" state from a Linear project
- Creates a git branch for each ticket
- Runs ralph loop with specified iterations
- Updates ticket status as work progresses (Todo → In Progress → Done)
- Posts progress comments to tickets after each iteration
- Automatically creates pull requests when tickets are completed
- Escalates to a specified user on errors
- Supports resumability - can resume from last processed ticket

**Linear Configuration File:**

Create a TOML config file (e.g., `linear.toml`):

```toml
# Linear API token (Bearer token for authentication)
# Get your API key from: https://linear.app/settings/api
token = "your-linear-api-token"

# Project ID to filter tickets (must be project UUID, not slug)
# Use --tickets command to list projects and get the UUID
project = "project-uuid-here"

# Linear username of user to tag on errors
escalate_user = "username"

# Base branch to create feature branches from (optional)
# If not specified, defaults to "main" or "master" (tries main first)
# Examples: "main", "master", "develop", "trunk"
base_branch = "main"
```

**Manager Mode Workflow:**
1. Validates git remote and GitHub CLI setup
2. Fetches highest priority ticket in "Todo" state
3. Creates git branch: `linear/{issue-id}-{slugified-title}`
4. Adds comment to ticket with branch name
5. Updates ticket to "In Progress"
6. Creates PRD from ticket title and description
7. Runs ralph loop for specified iterations
8. Posts progress updates after each iteration
9. On success:
   - Pushes branch to remote
   - Creates pull request with ticket information
   - Updates ticket to "Done" with PR link
   - Continues to next ticket
9. On error: Adds error comment, tags escalate_user, moves ticket back to "Todo", and exits

### How It Works

1. **First Run**: Ralph reads `.ralph/PRD.md` and begins working through incomplete tasks
2. **Each Iteration**: Executes all 6 steps (or 5 if not a 5th iteration, since Step Self-Improvement runs every 5th iteration)
3. **State Management**: Saves progress after each step, allowing resume if interrupted
4. **Completion**: Stops when PRD is complete or iteration limit is reached
5. **Blockers**: If Ralph encounters a blocker, it stops and reports the issue

The executable will first check for files in the `.ralph` directory. If found, they override the built-in defaults. If not found, the standard prompts are used.

## Project Structure

```
ralph-go/
├── .ralph/              # Optional: Configuration directory for overrides
│   ├── PRD.md           # Required: Product Requirements Document (not needed for manager mode)
│   ├── PROGRESS.md      # Optional: Progress tracking (auto-generated)
│   ├── PLAN.md          # Optional: Current plan (auto-generated, removed after completion)
│   ├── BACKLOG.md       # Optional: Critical issues backlog (auto-generated)
│   ├── ralph-state.txt  # Auto-generated: State for regular ralph mode
│   ├── manager-state.txt # Auto-generated: State for manager mode resume
│   └── *.txt            # Optional: Custom prompt files
├── main.go              # Main entry point
├── prompts.go           # Built-in prompts and prompt management
├── steps.go             # Step execution logic
├── claude.go            # Claude AI integration
├── state.go             # State persistence and resume logic
├── manager.go           # Linear manager mode implementation
├── config.go            # Configuration constants
├── prd.go               # PRD creation and initialization
└── README.md
```

The `.ralph` directory is optional for configuration. If present, files in this directory override the built-in standard prompts and configuration.

## Development

### Building from Source

```bash
# Build the executable
go build -o dist/ralph
```

### Project Architecture

- **main.go** - Entry point, orchestrates the Ralph loop and handles CLI arguments
- **prompts.go** - Manages built-in prompts and custom prompt loading
- **steps.go** - Implements each step of the Ralph workflow with retry logic
- **claude.go** - Wraps the Claude CLI tool for AI interactions
- **state.go** - Handles state persistence and resume functionality
- **manager.go** - Linear API integration and manager mode implementation
- **config.go** - Defines timeouts, retry limits, and required files
- **prd.go** - Handles PRD creation and initialization via `--init` flag

### Key Design Decisions

- **State Persistence**: Progress is saved after each step, allowing graceful recovery from interruptions
- **Timeout Handling**: Each step has configurable timeouts with automatic retries
- **Prompt Override System**: Built-in prompts can be overridden via `.ralph` directory for customization
- **Autonomous Operation**: System prompt enforces autonomous decision-making without asking for confirmation

## Contributing

Contributions are welcome! Here's how you can help:

### Reporting Issues

- Check existing issues before creating a new one
- Provide clear descriptions, steps to reproduce, and expected vs actual behavior
- Include relevant code snippets or error messages

### Submitting Changes

1. **Fork the repository** and create a feature branch
2. **Make your changes** following the existing code style
3. **Test your changes** by building and running the executable
4. **Update documentation** if you're adding features or changing behavior
5. **Submit a pull request** with a clear description of your changes

### Code Style Guidelines

- Follow Go standard formatting (`gofmt`)
- Keep functions focused and under 200-300 lines
- Add comments for exported functions and complex logic
- Maintain consistency with existing patterns
- Write clear, descriptive variable and function names

### Areas for Contribution

- **Bug fixes** - Fix issues reported in the issue tracker
- **Feature enhancements** - Add new capabilities or improve existing ones
- **Documentation** - Improve README, add examples, or clarify usage
- **Testing** - Add tests for existing functionality
- **Performance** - Optimize code execution or reduce resource usage
- **Error handling** - Improve error messages and recovery mechanisms

### Development Workflow

1. Create a branch for your changes
2. Make your changes and test them
3. Ensure the code builds successfully: `go build -o dist/ralph`
4. Update relevant documentation
5. Submit a pull request with a clear description

Thank you for contributing!

## Special Thanks

Special thanks to [Geoffrey Huntley](https://x.com/GeoffreyHuntley) and [Ryan Carson](https://x.com/ryancarson) for the inspiration for this project.

## License

MIT License - see [LICENSE](LICENSE) file for details.
