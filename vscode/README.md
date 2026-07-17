# ctx — Context Handoff

[![Version](https://img.shields.io/badge/version-1.0.0-blue.svg)](CHANGELOG.md)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

> Cut AI-assisted development handoff time from 30–45 minutes to under 5–10 minutes.

`ctx` exports the working context of an AI-assisted development session — git state, modified files, prompt history, and a structured AI summary — into a portable `.ctx` bundle that another developer (or another AI agent) can pick up with minimal onboarding.

This extension wraps the [`ctx` CLI](https://github.com/RiddhiKatarki/ctx) with a native VS Code experience: command-palette flows, a tree view of bundles in your workspace, rich WebView panels for inspecting summaries, and integrated `git apply` for handing off patches.

## Features

- **Export** the current workspace into a `.ctx` bundle via a guided multi-step flow (prompt source, summary provider, secret scan, content embedding).
- **Inspect** a bundle in a rich WebView that renders all 9 canonical summary sections (Current Objective, Completed Work, Remaining Tasks, Known Bugs, Architecture Decisions, Files To Read First, Previous Failed Approaches, Suggested Next Prompt, Estimated Reading Time).
- **Import** bundles with optional extraction and a one-click `git apply` of the bundled patch.
- **Bundle tree** in the activity bar lists every `.ctx` file in the workspace with branch, file count, and dirty indicator.
- **Prompt providers** auto-detected: Claude Code, OpenCode, Cursor (with SQLite extraction), Aider, or your own JSON file.
- **Local-first**: no cloud dependencies, no user accounts, no telemetry. The only network call is to your own OpenAI-compatible endpoint if you opt into the LLM summary provider.

## Quick start

1. Open a git repository in VS Code.
2. Run **ctx: Export Workspace Context** from the Command Palette.
3. Pick a prompt source (or accept *Auto-detect*), choose the template or OpenAI summary provider, and toggle secret-scan / content-embedding.
4. The bundle appears in your explorer with a `CTX` badge and in the **Context Bundles** tree.

To consume a bundle a teammate sent you:

1. Run **ctx: Import Bundle...** (or right-click a `.ctx` file in the explorer).
2. Choose *Validate only* or *Extract to folder*.
3. After extraction, click *Apply patch* to run `git apply patch.diff` in the integrated terminal.

## Commands

| Command | Description |
|---|---|
| `ctx: Export Workspace Context` | Capture the current state into a `.ctx` bundle |
| `ctx: Import Bundle...` | Validate and optionally extract a `.ctx` bundle |
| `ctx: Inspect Bundle` | Open the 9-section summary in a WebView |
| `ctx: Show Bundle Metadata` | Open manifest/metadata/git/files in a WebView |
| `ctx: Apply Patch from Bundle` | `git apply` the `patch.diff` from a bundle |

All commands are also available via:
- Explorer context menu on `.ctx` files
- Editor title bar when a `.ctx` file is open
- Inline buttons on bundle tree rows

## Configuration

| Key | Default | Description |
|---|---|---|
| `ctx.summaryProvider` | `"template"` | `"template"` (offline) or `"openai"` |
| `ctx.openaiBaseUrl` | `"https://api.openai.com/v1"` | Any OpenAI-compatible endpoint (Venice, Ollama, vLLM, etc.) |
| `ctx.openaiModel` | `"gpt-4o"` | Model name when using the `openai` provider |
| `ctx.defaultOutputName` | `"project.ctx"` | Default bundle filename |
| `ctx.defaultOutdir` | `".ctx"` | Default extraction directory on import |
| `ctx.secretScanEnabled` | `true` | Scan file contents for secrets and redact them before bundling |
| `ctx.includeContents` | `false` | Embed file contents for a self-contained bundle |
| `ctx.contentsThreshold` | `262144` | Max bytes per embedded file (256 KiB) |
| `ctx.showNotifications` | `"errorsOnly"` | `"all"`, `"errorsOnly"`, or `"none"` |

The OpenAI API key is **never** stored in `settings.json`. On first use of the `openai` provider, the extension prompts for it and stores it via VS Code's SecretStorage.

## Bundle format

A `.ctx` file is a ZIP archive containing 7 canonical files:

```
manifest.json    # Bundle version and provenance
metadata.json    # Project name, branch, OS, timestamps
git.json         # Branch, HEAD, dirty flag, remote URL, tag
summary.md       # 9-section structured project summary
prompts.json     # Recent prompt history
files.json       # List of relevant project files
patch.diff       # Uncommitted changes (git diff)
```

Self-contained bundles additionally embed file contents under `contents/`.

## Architecture

This extension ships prebuilt `ctx` binaries for **linux, darwin, and windows on amd64 and arm64** inside the `.vsix`. At runtime it spawns the binary for the current platform and consumes its machine-readable `--json` and `--stream` output.

The CLI's [stable exit-code contract](https://github.com/RiddhiKatarki/ctx/blob/main/internal/clierr/clierr.go) is the foundation of the extension's error UX:

| Exit | Code | Extension behavior |
|---|---|---|
| 0 | — | success |
| 1 | `user_error` | warning notification |
| 2 | `system_error` | error notification + *Open Logs* action |
| 3 | `invalid_bundle` | error notification with bundle-specific copy |

## Telemetry

None. The extension does not phone home and does not collect any usage data.

## License

[MIT](LICENSE)
