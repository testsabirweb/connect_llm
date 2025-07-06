# Taskmaster Guide for ConnectLLM

This project uses Taskmaster AI for intelligent task management. Here's a quick guide to get you started.

## Quick Start

Taskmaster has been initialized with 15 tasks based on the Product Requirements Document (PRD). The PRD is located at `.taskmaster/docs/prd.txt`.

## API Configuration

### Using DeepSeek AI

This project is configured to use DeepSeek AI for task management through OpenRouter. The API key is configured in `.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "taskmaster-ai": {
      "env": {
        "OPENROUTER_API_KEY": "your-deepseek-api-key"
      }
    }
  }
}
```

The main AI model is set to `deepseek/deepseek-chat-v3-0324` via OpenRouter.

### For CLI Usage

If you're using Taskmaster via the command line (not through Cursor), create a `.env` file in the project root:

```bash
OPENROUTER_API_KEY=your-openrouter-api-key-here
```

### Checking Model Configuration

```bash
# View current model configuration
tm models

# Switch to a different model if needed
tm models --set-main <model-id>
```

## Essential Commands

### View Project Status

```bash
# See all tasks with their current status
tm get-tasks

# Get detailed view of a specific task
tm get-task 1

# Find the next task to work on (considers dependencies)
tm next
```

### Work on Tasks

```bash
# Start working on a task
tm set-status 1 in-progress

# Mark a task as complete
tm set-status 1 done

# Update multiple task statuses at once
tm set-status 1,2,3 done
```

### Task Management

```bash
# Add a new task
tm add-task --prompt "Add comprehensive error handling to CSV parser"

# Update task details
tm update-task 3 --prompt "Include support for Unicode characters in CSV parsing"

# Add subtasks to break down complex work
tm expand 6 --num 5  # Creates 5 subtasks for task 6

# View task dependencies
tm validate-deps
```

## Task Organization

The project tasks are organized by priority and phase:

### High Priority (Foundation - Tasks 1-6)

- Task 1: Set up Golang project structure
- Task 2: Configure Weaviate vector database
- Task 3: Implement CSV data parser
- Task 4: Build document processing pipeline
- Task 5: Create data ingestion service
- Task 6: Implement search API endpoints

### Medium Priority (AI & Frontend - Tasks 7-10)

- Task 7: Set up Ollama integration
- Task 8: Build chat service backend
- Task 9: Create React frontend application
- Task 10: Implement authentication system

### Low Priority (Enhancements - Tasks 11-15)

- Task 11: Add search result ranking and filtering
- Task 12: Create monitoring and logging system
- Task 13: Implement data retention and cleanup
- Task 14: Add search analytics dashboard
- Task 15: Prepare connector framework

## Working with Subtasks

For complex tasks, you can create subtasks:

```bash
# Expand a task into subtasks
tm expand 1 --num 5 --prompt "Break down Golang setup into detailed steps"

# View task with subtasks
tm get-task 1

# Update subtask status
tm set-status 1.1 done
tm set-status 1.2 in-progress
```

## Tracking Progress

```bash
# See completion statistics
tm get-tasks  # Shows completion percentage at the bottom

# Filter by status
tm get-tasks --status pending
tm get-tasks --status in-progress
tm get-tasks --status done
```

## Advanced Features

### Research Integration

```bash
# Get research-backed information for a task
tm research --query "Best practices for Weaviate schema design" --saveTo 2
```

### Task Dependencies

```bash
# Add a dependency
tm add-dep 8 7  # Task 8 now depends on task 7

# Remove a dependency
tm remove-dep 8 7

# Check for circular dependencies
tm validate-deps
```

### Tags for Different Contexts

```bash
# Create a new tag for experimental features
tm add-tag experimental --copyFromCurrent

# Switch to the experimental tag
tm use-tag experimental

# List all tags
tm list-tags
```

## Tips

1. **Start with `tm next`** - It automatically finds the best task to work on based on dependencies
2. **Update tasks as you work** - Use `tm update-task` to add notes, discoveries, or changes
3. **Break down complex tasks** - Use `tm expand` when a task feels too large
4. **Check the PRD** - Review `.taskmaster/docs/prd.txt` for detailed requirements
5. **Use shell aliases** - `tm` is aliased to `taskmaster` for convenience

## Getting Help

```bash
# View all available commands
tm --help

# Get help for a specific command
tm get-task --help
```

Remember: Taskmaster helps you stay organized and focused on what matters most. Use it to track progress, manage dependencies, and ensure nothing falls through the cracks!
