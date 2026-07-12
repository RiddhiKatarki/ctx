# ctx — Context Handoff

A cross-platform CLI tool that allows developers to export and import the **working context** of an AI-assisted software development session.

`ctx` serializes the current development state (git metadata, modified files, prompt history, AI-generated summary) into a portable `.ctx` bundle so another developer or AI agent can continue working with minimal onboarding.

**Goal:** Reduce project handoff time from ~30-45 minutes to under 5-10 minutes.

## Core Principles

- Platform agnostic
- Git-first
- Local-first (no cloud dependencies in V1)
- No user accounts
- No IDE plugins required
- Portable bundle format
- Extensible architecture

## Install

### Build from source

```bash
git clone <repo-url>
cd ctx
go build -o ctx ./cmd/ctx
# Move binary to your PATH
mv ctx /usr/local/bin/
```

### Verify

```bash
ctx --version
```

## Quick Start

### Export your context

```bash
# From within a git repository
ctx export

# With prompt history and custom output
ctx export --prompts prompts.json --output my-bundle.ctx

# With extra files included
ctx export --files README.md,docs/architecture.md

# Use an LLM for richer summaries
ctx export --summary-provider openai --api-key $OPENAI_API_KEY
```

This creates `project.ctx` — a single portable file.

### Inspect a bundle

```bash
ctx inspect project.ctx
```

Displays a readable summary without extracting:

```
Project: Backend API

Branch:
  feature/auth

Current Goal:
  Implement streaming support for the /events endpoint

Modified Files:
  4

Known Bug:
  Timeout after 30 seconds when client disconnects mid-stream

Estimated Reading Time:
  ~3 minutes
```

### Import a bundle

```bash
# Validate and display summary
ctx import project.ctx

# Extract to a directory for reference
ctx import project.ctx --outdir .ctx/

# Then apply the uncommitted changes if needed
git apply .ctx/patch.diff
```

## Commands

### `ctx export`

Captures project context and creates a `.ctx` bundle.

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--output, -o` | `project.ctx` | Output bundle path |
| `--project-name` | repo dir name | Override project name |
| `--prompts` | (none) | Path to JSON file with prompt history |
| `--files` | (none) | Comma-separated extra file paths to include |
| `--summary-provider` | `template` | Summary provider: `template` or `openai` |
| `--api-key` | (none) | API key for LLM summary provider |
| `--api-base-url` | `https://api.openai.com/v1` | Base URL for LLM API |
| `--model` | `gpt-4o` | LLM model name |

### `ctx inspect [path]`

Displays summary info from a `.ctx` bundle without extracting.

### `ctx import [path]`

Extracts and validates a `.ctx` bundle.

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--outdir` | (none) | Directory to extract bundle contents to |

## Bundle Format

A `.ctx` file is a ZIP archive containing:

```
manifest.json    # Bundle version and provenance
metadata.json    # General project metadata
git.json         # Git state (branch, HEAD, dirty, remote, tag)
summary.md       # AI-generated structured project summary
prompts.json     # Recent prompt history
files.json       # List of relevant project files
patch.diff       # Uncommitted changes (git diff)
```

### summary.md sections

- Current Objective
- Completed Work
- Remaining Tasks
- Known Bugs
- Architecture Decisions
- Files To Read First
- Previous Failed Approaches
- Suggested Next Prompt
- Estimated Reading Time

## Architecture

```
cmd/
    ctx/
        main.go            # Cobra CLI entry point

internal/
    archive/               # ZIP (.ctx) create/extract/peek
    bundle/                # In-memory bundle model + serialization
    export/                # Export orchestration
    import/                # Import orchestration
    inspect/               # Inspect orchestration
    git/                   # GitProvider interface + CLI implementation
    summary/               # SummaryProvider interface + template + OpenAI
    schema/                # Version constants, archive filenames, validation
    providers/             # PromptProvider interface + mock + file

pkg/
    types/                 # Shared domain types (no internal deps)

examples/                  # Sample prompts and bundles
```

### Extensible Interfaces

The tool relies on interfaces so providers can be swapped later:

**GitProvider** — currently uses Git CLI; can be replaced with go-git.

**PromptProvider** — currently mock or file-based; future: Claude Code, Cursor, OpenCode, Windsurf, Aider.

**SummaryProvider** — currently template (local) or OpenAI-compatible; future: Anthropic, Venice, Ollama, local models.

## Security

V1 automatically excludes files matching secret patterns:

- `.env`, `.env.local`
- `*.pem`, `*.key`
- `id_rsa`, `id_rsa.pub`
- `*.p12`, `*.pfx`

These are skipped with a log message. Future versions will include full secret scanning and user-configurable exclusions.

## Development

```bash
# Build
go build ./...

# Test
go test ./...

# Vet
go vet ./...

# Build binary
go build -o ctx ./cmd/ctx/
```

## License

MIT
