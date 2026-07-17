# Change Log

All notable changes to the **ctx** VS Code extension will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2026-07-17

Initial public release. Wraps the `ctx` CLI v2.1.0 with a native VS Code experience.

### Added
- **Commands**: `ctx: Export Workspace Context`, `ctx: Import Bundle...`, `ctx: Inspect Bundle`, `ctx: Show Bundle Metadata`, `ctx: Apply Patch from Bundle`, `ctx: Refresh Bundles`.
- **Tree view**: "Context Bundles" in the activity bar with auto-refresh on `.ctx` file changes, inline Inspect/Info buttons, and a welcome view for empty states.
- **WebView panels**: rich rendering of the 9-section project summary (Inspect) and manifest/metadata/git/files (Info) using VS Code theme variables.
- **Export flow**: multi-step QuickPick for prompt source (auto/file/claudecode/opencode/cursor/aider/mock), summary provider (template/openai), secret-scan and content-embedding toggles, output path. Progress notification driven by NDJSON events.
- **Import flow**: file picker, validate-only vs. extract-to-folder choice, Apply Patch action that runs `git apply` in an integrated terminal.
- **File decoration**: subtle `CTX` badge on `.ctx` files in the explorer.
- **Configuration**: 9 settings covering summary provider, OpenAI endpoint/model, defaults, secret scanning, content embedding, and notification verbosity.
- **SecretStorage**: OpenAI API key never touches `settings.json`.
- **Bundled binaries**: prebuilt `ctx` binaries for linux/darwin/windows on amd64/arm64 (12–13 MB each, ~35 MB total in the `.vsix`).
- **Version check**: extension warns on activation if the bundled binary is older than the supported minimum.
- **Error UX**: exit-code-aware notification routing (warning for user errors, error + *Open Logs* for system errors, error for invalid bundles).

### Bundled CLI version
- `ctx` v2.1.0 — adds Cursor SQLite prompt extraction and the `ctx version --json` subcommand.

## [Unreleased]

_See the `main` branch on GitHub._
