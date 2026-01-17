#!/bin/bash
# afk-ralph.sh - fully autonomous loop with Orb

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [ -z "$1" ]; then
  echo "Usage: $0 <iterations>"
  exit 1
fi

# Change to script directory to ensure relative paths work
cd "$SCRIPT_DIR"

# Verify required files exist
if [ ! -f "PRD.md" ]; then
  echo "❌ Error: PRD.md not found in $SCRIPT_DIR" >&2
  exit 1
fi

if [ ! -f "progress.txt" ]; then
  echo "❌ Error: progress.txt not found in $SCRIPT_DIR" >&2
  exit 1
fi

# State file for resume functionality
STATE_FILE="ralph-state.txt"

# Timeout configuration (in seconds)
TIMEOUT_PLANNING=1800          # 30 minutes for planning
TIMEOUT_IMPLEMENTATION=3600    # 60 minutes for implementation
TIMEOUT_CLEANUP=900            # 15 minutes for cleanup
TIMEOUT_SELF_IMPROVEMENT=1800  # 30 minutes for self-improvement analysis
TIMEOUT_COMMIT=300             # 5 minutes for commit

MAX_RETRIES=3

# Initialize resume variables
RESUME_FROM_ITERATION=""
RESUME_FROM_STEP=""

# State management functions
load_state() {
  if [ ! -f "$STATE_FILE" ]; then
    return 1
  fi
  
  # Read state values
  while IFS='=' read -r key value; do
    case "$key" in
      iteration)
        RESUME_ITERATION="$value"
        ;;
      max_iterations)
        RESUME_MAX_ITERATIONS="$value"
        ;;
      current_step)
        RESUME_CURRENT_STEP="$value"
        ;;
      last_completed_step)
        RESUME_LAST_COMPLETED_STEP="$value"
        ;;
    esac
  done < "$STATE_FILE"
  
  return 0
}

save_state() {
  local iteration=$1
  local max_iterations=$2
  local current_step=$3
  local last_completed_step=$4
  
  cat > "$STATE_FILE" <<EOF
iteration=$iteration
max_iterations=$max_iterations
current_step=$current_step
last_completed_step=$last_completed_step
EOF
}

clear_state() {
  if [ -f "$STATE_FILE" ]; then
    rm -f "$STATE_FILE"
  fi
}

detect_resume() {
  if [ ! -f "$STATE_FILE" ]; then
    return 1
  fi
  
  # Load state
  if ! load_state; then
    echo "⚠️  State file exists but could not be read. Starting fresh." >&2
    clear_state
    return 1
  fi
  
  # Validate state
  if [ -z "$RESUME_ITERATION" ] || [ -z "$RESUME_MAX_ITERATIONS" ]; then
    echo "⚠️  State file is corrupted. Starting fresh." >&2
    clear_state
    return 1
  fi
    
  # Check if iteration exceeds max
  if [ "$RESUME_ITERATION" -gt "$RESUME_MAX_ITERATIONS" ]; then
    echo "⚠️  State file indicates iteration exceeds max. Starting fresh." >&2
    clear_state
    return 1
  fi
  
  # Determine resume step based on PLAN.md existence
  local resume_step
  local step_name
  if [ -f "PLAN.md" ]; then
    resume_step=2
    step_name="Step 2 (Implementation)"
  elif [ -n "$RESUME_LAST_COMPLETED_STEP" ] && [ "$RESUME_LAST_COMPLETED_STEP" -ge 2 ]; then
    resume_step=3
    step_name="Step 3 (Cleanup)"
  else
    resume_step=1
    step_name="Step 1 (Planning)"
  fi
  
  # Prompt user
  echo "🔄 Resume detected:"
  echo "   Iteration: $RESUME_ITERATION/$RESUME_MAX_ITERATIONS"
  echo "   Resume from: $step_name"
  echo ""
  read -p "Continue from here? (y/n): " -n 1 -r
  echo ""
  
  if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Starting fresh..."
    clear_state
    return 1
  fi
  
  # Set resume variables
  RESUME_FROM_ITERATION=$RESUME_ITERATION
  RESUME_FROM_STEP=$resume_step
  
  return 0
}

# Resume detection at startup
MAX_ITERATIONS=$1
START_ITERATION=1

if detect_resume; then
  START_ITERATION=$RESUME_FROM_ITERATION
  echo "✅ Resuming from iteration $START_ITERATION, step $RESUME_FROM_STEP"
else
  echo "🚀 Starting fresh"
fi

# Cleanup function for exit handlers
# Note: We don't clear state here - state is cleared explicitly on successful completion
# This allows state to persist on errors/blocks so user can resume
cleanup_on_exit() {
  # Just ensure temp files are cleaned up
  # State file is managed explicitly in completion paths
  :
}

for ((i=START_ITERATION; i<=MAX_ITERATIONS; i++)); do
  echo "🔄 Iteration $i/$MAX_ITERATIONS"
  
  # Save state at iteration start
  save_state $i $MAX_ITERATIONS 1 0

  # Use a temp file to capture output while still streaming to stdout
  TEMP_OUTPUT=$(mktemp) || {
    echo "❌ Error: Failed to create temporary file" >&2
    exit 1
  }
  
  # Set up trap to clean up temp file on exit
  trap 'rm -f "$TEMP_OUTPUT"' EXIT INT TERM
  
  # Determine if we should skip to a later step (resume)
  SKIP_TO_STEP=${RESUME_FROM_STEP:-0}
  if [ $i -eq $START_ITERATION ] && [ -n "$RESUME_FROM_STEP" ] && [ $RESUME_FROM_STEP -gt 1 ]; then
    SKIP_TO_STEP=$RESUME_FROM_STEP
  else
    SKIP_TO_STEP=0
    RESUME_FROM_STEP=""  # Clear after first iteration
  fi
  
  # Step 1: Planning
  if [ $SKIP_TO_STEP -le 1 ]; then
    STEP1_RETRY_COUNT=0
    STEP1_SUCCESS=false
    while [ $STEP1_RETRY_COUNT -lt $MAX_RETRIES ] && [ "$STEP1_SUCCESS" = false ]; do
      if [ $STEP1_RETRY_COUNT -gt 0 ]; then
        echo ""
        echo "🔄 Retrying Step 1 (attempt $((STEP1_RETRY_COUNT + 1))/$MAX_RETRIES)..."
      else
        echo ""
        echo "📋 Step 1: Planning... (timeout: ${TIMEOUT_PLANNING}s)"
      fi
      save_state $i $MAX_ITERATIONS 1 0
      if timeout $TIMEOUT_PLANNING claude --system-prompt "$(cat AUTONOMOUS_SYSTEM_PROMPT.md)" --dangerously-skip-permissions --no-session-persistence -p "@PRD.md @progress.txt \
1. Review all incomplete tasks in PRD and assess their complexity (easy, medium, hard). \
2. PRIORITY: Find an incomplete task that is EASY or MEDIUM complexity. \
3. If no easy/medium tasks exist: \
   a. Select a MEDIUM-HARD complexity task \
   b. Break it down into 3-5 smaller, manageable subtasks (each should be easy or medium complexity) \
   c. Update PRD.md by replacing the original task with the subtasks (maintain the same checkbox format) \
   d. Select ONE of the newly created subtasks to work on \
4. Create a detailed plan for the selected task using "megathink" mode. Make sure to include vitests, detailed task breakdown and acceptance criteria. \
5. Write the plan to PLAN.md. \
ONLY WORK ON ONE TASK. \
DO NOT ask which task to work on - select one autonomously using the decision-making framework. \
Proceed immediately to planning - do not ask for confirmation. \
If PRD is complete, output <promise>COMPLETE</promise>. \
If you are blocked, output <promise>BLOCKED</promise> and explain the blocker." 2>&1 | tee "$TEMP_OUTPUT"; then
        STEP1_SUCCESS=true
      else
        timeout_exit_code=$?
        if [ $timeout_exit_code -eq 124 ]; then
          STEP1_RETRY_COUNT=$((STEP1_RETRY_COUNT + 1))
          if [ $STEP1_RETRY_COUNT -ge $MAX_RETRIES ]; then
            echo "⏱️  Step 1 timed out after ${MAX_RETRIES} attempts"
            save_state $i $MAX_ITERATIONS 1 0
            rm -f "$TEMP_OUTPUT"
            trap - EXIT INT TERM
            exit 0
          else
            echo "⏱️  Step 1 timed out after ${TIMEOUT_PLANNING}s, will retry..."
          fi
        else
          echo "❌ Step 1 failed with exit code $timeout_exit_code"
          save_state $i $MAX_ITERATIONS 1 0
          rm -f "$TEMP_OUTPUT"
          trap - EXIT INT TERM
          exit 0
        fi
      fi
    done

    result=$(cat "$TEMP_OUTPUT")
    if [[ "$result" == *"<promise>COMPLETE</promise>"* ]]; then
      echo "✅ PRD complete after $i iterations!"
      save_state $i $MAX_ITERATIONS 0 1
      rm -f "$TEMP_OUTPUT"
      trap - EXIT INT TERM
      clear_state
      exit 0
    fi

    if [[ "$result" == *"<promise>BLOCKED</promise>"* ]]; then
      echo "❌ Ralph is blocked during planning, please fix the blocker and run again."
      save_state $i $MAX_ITERATIONS 1 0
      rm -f "$TEMP_OUTPUT"
      trap - EXIT INT TERM
      # Exit with code 0 but keep state file for resume
      exit 0
    fi
    
    # Step 1 completed successfully
    save_state $i $MAX_ITERATIONS 2 1
  else
    echo "⏭️  Step 1: Skipping (resuming from later step)"
  fi

  # Step 2: Implementation and Validation
  if [ $SKIP_TO_STEP -le 2 ]; then
    STEP2_RETRY_COUNT=0
    STEP2_SUCCESS=false
    while [ $STEP2_RETRY_COUNT -lt $MAX_RETRIES ] && [ "$STEP2_SUCCESS" = false ]; do
      if [ $STEP2_RETRY_COUNT -gt 0 ]; then
        echo ""
        echo "🔄 Retrying Step 2 (attempt $((STEP2_RETRY_COUNT + 1))/$MAX_RETRIES)..."
      else
        echo ""
        echo "🔨 Step 2: Implementation and Validation... (timeout: ${TIMEOUT_IMPLEMENTATION}s)"
      fi
      save_state $i $MAX_ITERATIONS 2 1
      if timeout $TIMEOUT_IMPLEMENTATION claude --system-prompt "$(cat AUTONOMOUS_SYSTEM_PROMPT.md)" --dangerously-skip-permissions --no-session-persistence -p "@PRD.md @PLAN.md @progress.txt @CLAUDE.md \
1. Pay close attention to @CLAUDE.md and follow any instructions it provides. \
2. Implement the task completely, based on PLAN.md. \
3. Run tests and type checks. Fix ALL errors and warnings. \
4. Ensure test coverage is at least 80%. \
5. Run a code review and fix ALL issues. \
If PLAN.md is ambiguous, interpret it reasonably and proceed - do not ask for clarification. \
Complete the implementation fully - do not ask if you should continue or what to do next. \
If you are blocked, output <promise>BLOCKED</promise> and explain the blocker." 2>&1 | tee "$TEMP_OUTPUT"; then
        STEP2_SUCCESS=true
      else
        timeout_exit_code=$?
        if [ $timeout_exit_code -eq 124 ]; then
          STEP2_RETRY_COUNT=$((STEP2_RETRY_COUNT + 1))
          if [ $STEP2_RETRY_COUNT -ge $MAX_RETRIES ]; then
            echo "⏱️  Step 2 timed out after ${MAX_RETRIES} attempts"
            save_state $i $MAX_ITERATIONS 2 1
            rm -f "$TEMP_OUTPUT"
            trap - EXIT INT TERM
            exit 0
          else
            echo "⏱️  Step 2 timed out after ${TIMEOUT_IMPLEMENTATION}s, will retry..."
          fi
        else
          echo "❌ Step 2 failed with exit code $timeout_exit_code"
          save_state $i $MAX_ITERATIONS 2 1
          rm -f "$TEMP_OUTPUT"
          trap - EXIT INT TERM
          exit 0
        fi
      fi
    done

    result=$(cat "$TEMP_OUTPUT")
    if [[ "$result" == *"<promise>BLOCKED</promise>"* ]]; then
      echo "❌ Ralph is blocked during implementation, please fix the blocker and run again."
      save_state $i $MAX_ITERATIONS 2 1
      rm -f "$TEMP_OUTPUT"
      trap - EXIT INT TERM
      # Exit with code 0 but keep state file for resume
      exit 0
    fi
    
    # Step 2 completed successfully
    save_state $i $MAX_ITERATIONS 3 2
  else
    echo "⏭️  Step 2: Skipping (resuming from later step)"
  fi

  # Step 3: Cleanup and Documentation
  if [ $SKIP_TO_STEP -le 3 ]; then
    STEP3_RETRY_COUNT=0
    STEP3_SUCCESS=false
    while [ $STEP3_RETRY_COUNT -lt $MAX_RETRIES ] && [ "$STEP3_SUCCESS" = false ]; do
      if [ $STEP3_RETRY_COUNT -gt 0 ]; then
        echo ""
        echo "🔄 Retrying Step 3 (attempt $((STEP3_RETRY_COUNT + 1))/$MAX_RETRIES)..."
      else
        echo ""
        echo "🧹 Step 3: Cleanup and Documentation... (timeout: ${TIMEOUT_CLEANUP}s)"
      fi
      save_state $i $MAX_ITERATIONS 3 2
      if timeout $TIMEOUT_CLEANUP claude --system-prompt "$(cat AUTONOMOUS_SYSTEM_PROMPT.md)" --dangerously-skip-permissions --no-session-persistence -p "@PRD.md @PLAN.md @progress.txt \
1. Update PRD.md with the completed task. \
2. Remove PLAN.md. \
3. Update progress.txt with any learnings. \
4. Update @CLAUDE.md with any new features or changes. Ensure to use CLAUDE.md best practices and conventions: high‑level project context, clear guardrails, key commands, and links to deeper docs, while avoiding long prose and unnecessary detail. \
5. Update @README.md with any applicable changes. Update README.md only if: 1) New features were added that users should know about, 2) Setup/installation steps changed, 3) Configuration options were added/removed. \
If no README updates are needed, skip that step - do not ask." 2>&1 | tee "$TEMP_OUTPUT"; then
        STEP3_SUCCESS=true
      else
        timeout_exit_code=$?
        if [ $timeout_exit_code -eq 124 ]; then
          STEP3_RETRY_COUNT=$((STEP3_RETRY_COUNT + 1))
          if [ $STEP3_RETRY_COUNT -ge $MAX_RETRIES ]; then
            echo "⏱️  Step 3 timed out after ${MAX_RETRIES} attempts"
            save_state $i $MAX_ITERATIONS 3 2
            rm -f "$TEMP_OUTPUT"
            trap - EXIT INT TERM
            exit 0
          else
            echo "⏱️  Step 3 timed out after ${TIMEOUT_CLEANUP}s, will retry..."
          fi
        else
          echo "❌ Step 3 failed with exit code $timeout_exit_code"
          save_state $i $MAX_ITERATIONS 3 2
          rm -f "$TEMP_OUTPUT"
          trap - EXIT INT TERM
          exit 0
        fi
      fi
    done
    
    # Step 3 completed successfully
    save_state $i $MAX_ITERATIONS 4 3
  else
    echo "⏭️  Step 3: Skipping (resuming from later step)"
  fi

  # Step 4: Self-Improvement Analysis (every 5th iteration)
  if (( i % 5 == 0 )); then
    if [ $SKIP_TO_STEP -le 4 ]; then
      STEP4_RETRY_COUNT=0
      STEP4_SUCCESS=false
      while [ $STEP4_RETRY_COUNT -lt $MAX_RETRIES ] && [ "$STEP4_SUCCESS" = false ]; do
        if [ $STEP4_RETRY_COUNT -gt 0 ]; then
          echo ""
          echo "🔄 Retrying Step 4 (attempt $((STEP4_RETRY_COUNT + 1))/$MAX_RETRIES)..."
        else
          echo ""
          echo "🔍 Step 4: Self-Improvement Analysis (iteration $i)... (timeout: ${TIMEOUT_SELF_IMPROVEMENT}s)"
        fi
        save_state $i $MAX_ITERATIONS 4 3
        if timeout $TIMEOUT_SELF_IMPROVEMENT claude --system-prompt "$(cat AUTONOMOUS_SYSTEM_PROMPT.md)" --dangerously-skip-permissions --no-session-persistence -p "@PRD.md @progress.txt \
Analyze the codebase for improvements, but ONLY add CRITICAL and HIGH priority issues to BACKLOG.md. \
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
   - Missing functionality already tracked in PRD.md \
   - Technical debt that doesn't block features or cause bugs \
   - Performance optimizations for code paths that aren't bottlenecks \
   - Low/Medium priority issues (unless they're security-related) \
4. DEDUPLICATION: Before adding any issue to BACKLOG.md: \
   - Read BACKLOG.md and check for duplicates (similar issues already documented) \
   - Only add if it provides new critical information or indicates the issue is more severe than previously documented \
   - If a similar issue exists, update the existing entry rather than creating a duplicate \
5. Format BACKLOG.md with clear sections and markdown formatting for easy review. \
6. For each finding, document: \
   - Category (Security, Data Integrity, Production Blocker, Performance) \
   - Description with measurable impact \
   - Location (file path and relevant code sections) \
   - Priority (CRITICAL or HIGH only) \
   - Specific evidence of impact \
   - Suggested approach for addressing it \
7. If BACKLOG.md doesn't exist, create it. If it exists, append new findings (avoid duplicates). \
8. Organize findings by category and priority for easy integration into PRD.md later. \
9. If there are no CRITICAL or HIGH priority issues to add, output 'No critical issues found' and skip updating BACKLOG.md. \
Complete the analysis and update BACKLOG.md - do not ask for confirmation before adding items." 2>&1 | tee "$TEMP_OUTPUT"; then
          STEP4_SUCCESS=true
        else
          timeout_exit_code=$?
          if [ $timeout_exit_code -eq 124 ]; then
            STEP4_RETRY_COUNT=$((STEP4_RETRY_COUNT + 1))
            if [ $STEP4_RETRY_COUNT -ge $MAX_RETRIES ]; then
              echo "⏱️  Step 4 timed out after ${MAX_RETRIES} attempts"
              save_state $i $MAX_ITERATIONS 4 3
              rm -f "$TEMP_OUTPUT"
              trap - EXIT INT TERM
              exit 0
            else
              echo "⏱️  Step 4 timed out after ${TIMEOUT_SELF_IMPROVEMENT}s, will retry..."
            fi
          else
            echo "❌ Step 4 failed with exit code $timeout_exit_code"
            save_state $i $MAX_ITERATIONS 4 3
            rm -f "$TEMP_OUTPUT"
            trap - EXIT INT TERM
            exit 0
          fi
        fi
      done
      
      # Step 4 completed successfully
      save_state $i $MAX_ITERATIONS 5 4
    else
      echo "⏭️  Step 4: Skipping (resuming from later step)"
    fi
  else
    echo "⏭️  Step 4: Skipping self-improvement analysis (runs every 5th iteration)"
  fi

  # Step 5: Commit
  if [ $SKIP_TO_STEP -le 5 ] || [ $SKIP_TO_STEP -eq 0 ]; then
    STEP5_RETRY_COUNT=0
    STEP5_SUCCESS=false
    while [ $STEP5_RETRY_COUNT -lt $MAX_RETRIES ] && [ "$STEP5_SUCCESS" = false ]; do
      if [ $STEP5_RETRY_COUNT -gt 0 ]; then
        echo ""
        echo "🔄 Retrying Step 5 (attempt $((STEP5_RETRY_COUNT + 1))/$MAX_RETRIES)..."
      else
        echo ""
        echo "💾 Step 5: Commit... (timeout: ${TIMEOUT_COMMIT}s)"
      fi
      save_state $i $MAX_ITERATIONS 5 4
      if timeout $TIMEOUT_COMMIT claude --system-prompt "$(cat AUTONOMOUS_SYSTEM_PROMPT.md)" --dangerously-skip-permissions --no-session-persistence -p "@PRD.md @progress.txt \
Review the changes and commit with a clear message. \
Use format: 'feat: [brief description]' or 'fix: [brief description]' based on the changes. \
Review git status, stage all relevant changes, and commit - do not ask for approval. \
If there are no changes to commit, output 'No changes to commit' and proceed to next iteration." 2>&1 | tee "$TEMP_OUTPUT"; then
        STEP5_SUCCESS=true
      else
        timeout_exit_code=$?
        if [ $timeout_exit_code -eq 124 ]; then
          STEP5_RETRY_COUNT=$((STEP5_RETRY_COUNT + 1))
          if [ $STEP5_RETRY_COUNT -ge $MAX_RETRIES ]; then
            echo "⏱️  Step 5 timed out after ${MAX_RETRIES} attempts"
            save_state $i $MAX_ITERATIONS 5 4
            rm -f "$TEMP_OUTPUT"
            trap - EXIT INT TERM
            exit 0
          else
            echo "⏱️  Step 5 timed out after ${TIMEOUT_COMMIT}s, will retry..."
          fi
        else
          echo "❌ Step 5 failed with exit code $timeout_exit_code"
          save_state $i $MAX_ITERATIONS 5 4
          rm -f "$TEMP_OUTPUT"
          trap - EXIT INT TERM
          exit 0
        fi
      fi
    done
    
    # Step 5 completed successfully
    save_state $i $MAX_ITERATIONS 0 5
  else
    echo "⏭️  Step 5: Skipping (resuming from later step)"
  fi

  # Clean up temp file
  rm -f "$TEMP_OUTPUT"
  trap - EXIT INT TERM  # Remove trap after cleanup

  # Clear resume step after first iteration
  if [ $i -eq $START_ITERATION ]; then
    RESUME_FROM_STEP=""
  fi

  sleep 2  # Brief pause between iterations
done

echo "⚠️  Reached iteration limit ($MAX_ITERATIONS) but PRD not yet complete"
clear_state
exit 1