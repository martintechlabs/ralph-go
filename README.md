# ralph-go

A Go-based executable that runs a Ralph loop with configurable prompts and settings.

## Overview

This project provides a `ralph` executable that executes a Ralph loop. The executable comes with a standard set of prompts and configuration files built-in. These defaults can be overridden by placing corresponding files in a `.ralph` directory.

## Features

- Executes a Ralph loop
- Standard prompts included by default
- Configuration override: customize prompts and settings via `.ralph` directory
- Single executable deployment

## Configuration

The `ralph` executable includes a standard set of prompts and configuration files. To customize behavior, create a `.ralph` directory and place your configuration files there. Files in the `.ralph` directory will override the built-in defaults.

If a `.ralph` directory doesn't exist or specific files are missing, the executable will use its built-in defaults.

## Usage

```bash
# Run with default prompts (no configuration needed)
./ralph

# Run with custom configuration
# Create a .ralph directory and add your configuration files
mkdir .ralph
# Add your custom files to .ralph/
./ralph
```

The executable will first check for files in the `.ralph` directory. If found, they override the built-in defaults. If not found, the standard prompts are used.

## Project Structure

```
ralph-go/
├── .ralph/          # Optional: Configuration directory for overrides
├── README.md
└── ...              # Source code
```

The `.ralph` directory is optional. If present, files in this directory override the built-in standard prompts and configuration.

## Development

```bash
# Build the executable
go build -o dist/ralph
```

## License

[Add license information here]
