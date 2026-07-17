# ctx — Context Handoff

Ever finished a coding session with an AI assistant, then had to spend 30 minutes explaining to a teammate (or your future self) what you were working on? **Context Handoff fixes that.**

`ctx` captures the full state of an AI-assisted development session — what you were building, what's done, what's left, what files to look at first, what you already tried — and packs it into a single portable `.ctx` file. Hand it to a teammate, drop it in a chat, or save it for Monday morning. Anyone (or any AI agent) can pick up exactly where you left off in under 5 minutes.

## What's in a `.ctx` bundle?

A `.ctx` file is a small ZIP archive containing:

| File | What it captures |
|---|---|
| `summary.md` | A 9-section project summary (see below) |
| `git.json` | Branch, HEAD commit, dirty flag, remote URL, current tag |
| `patch.diff` | Your uncommitted changes as a `git diff` |
| `files.json` | List of modified files |
| `prompts.json` | Recent prompt history from your AI assistant |
| `metadata.json` | Project name, OS, timestamps |
| `manifest.json` | Bundle format version and provenance |

### The 9-section project summary

The heart of a bundle. Generated from your git state and prompt history (either locally or via an LLM):

1. **Current Objective** — what you're trying to build
2. **Completed Work** — what's already done
3. **Remaining Tasks** — what's left
4. **Known Bugs** — issues to watch out for
5. **Architecture Decisions** — context for the choices made
6. **Files To Read First** — where to look first
7. **Previous Failed Approaches** — what didn't work (so you don't repeat it)
8. **Suggested Next Prompt** — a ready-to-use prompt to continue
9. **Estimated Reading Time** — how long onboarding will take

## What can I do with this extension?

### Export your current session

Run **ctx: Export Workspace Context** from the Command Palette. A guided flow walks you through:

- **Prompt source** — auto-detect (Claude Code, OpenCode, Cursor, Aider), load from a JSON file, or skip
- **Summary provider** — local template (offline, instant) or any OpenAI-compatible LLM (OpenAI, Surplus, Ollama, vLLM, Venice) for a richer summary
- **Secret scanning** — redact API keys, tokens, and private keys before bundling
- **Content embedding** — optionally embed file contents so the bundle is fully self-contained

The resulting `project.ctx` lands in your workspace with a `CTX` badge, ready to share.

### Inspect a bundle

Right-click any `.ctx` file → **ctx: Inspect Bundle**. The full 9-section summary opens in a Markdown preview pane — readable, copyable, and searchable.

### Import a bundle

Right-click → **ctx: Import Bundle...**. Choose to validate-only or extract to a folder. After extraction, a one-click **Apply patch** button runs `git apply patch.diff` in the integrated terminal, so you can pick up your teammate's uncommitted changes verbatim.

### Browse bundles in your workspace

The activity bar gets a new **ctx** icon. Click it to see every `.ctx` file in your workspace with branch, file count, and dirty indicator at a glance. Click a bundle to inspect it.

## Commands

| Command | What it does |
|---|---|
| `ctx: Export Workspace Context` | Capture current state into a `.ctx` bundle |
| `ctx: Import Bundle...` | Validate / extract a `.ctx` bundle |
| `ctx: Inspect Bundle` | Open the 9-section summary |
| `ctx: Show Bundle Metadata` | Show manifest / metadata / git / files |
| `ctx: Apply Patch from Bundle` | `git apply` the `patch.diff` from a bundle |
| `ctx: Update OpenAI API Key` | Edit the stored API key |
| `ctx: Refresh Bundles` | Re-scan the workspace for `.ctx` files |

Commands surface in the Command Palette, the explorer context menu on `.ctx` files, the editor title bar, and inline on bundle tree rows.

## Configuration

| Setting | Default | Description |
|---|---|---|
| `ctx.summaryProvider` | `"template"` | `"template"` (offline) or `"openai"` (LLM) |
| `ctx.openaiBaseUrl` | `"https://api.openai.com/v1"` | Any OpenAI-compatible endpoint |
| `ctx.openaiModel` | `"gpt-4o"` | Model name for the `openai` provider |
| `ctx.defaultOutputName` | `"project.ctx"` | Default bundle filename |
| `ctx.defaultOutdir` | `".ctx"` | Default extraction directory on import |
| `ctx.secretScanEnabled` | `true` | Scan and redact secrets before bundling |
| `ctx.includeContents` | `false` | Embed file contents for self-contained bundles |
| `ctx.contentsThreshold` | `262144` | Max bytes per embedded file (256 KiB) |
| `ctx.showNotifications` | `"errorsOnly"` | `"all"`, `"errorsOnly"`, or `"none"` |

**API key handling:** Your OpenAI-compatible API key is stored via VS Code's SecretStorage — never in `settings.json`, never in git. Use the `ctx: Update OpenAI API Key` command to replace or delete it.

## Custom LLM providers

The summary provider works with **any** OpenAI-compatible endpoint. Some examples:

- **OpenAI** — `https://api.openai.com/v1` · model `gpt-4o`
- **Surplus** — `https://api.surplusintelligence.ai/v1` · model `glm-5.2`
- **Ollama (local)** — `http://localhost:11434/v1` · model `llama3`
- **vLLM** — `http://localhost:8000/v1` · model of your choice

When you run export and pick the OpenAI-compatible provider, you'll be prompted for the base URL and model name inline — no need to edit settings first.

## Privacy

- **Local-first.** The only network call is to your own LLM endpoint, and only if you opt into the LLM summary provider.
- **No telemetry.** The extension does not phone home and collects no usage data.
- **No accounts.** No sign-up, no login, no cloud service.
- **Secret-aware.** Files matching secret patterns (`.env`, `*.pem`, `id_rsa`, API keys, tokens) are excluded or redacted by default.

## How it works

This extension ships prebuilt `ctx` binaries for **Linux, macOS, and Windows on amd64 and arm64** inside the `.vsix` (~5 MB per platform). At runtime it spawns the binary for the current platform and consumes its machine-readable JSON output — no Node-side reimplementation of git, secret scanning, or bundle I/O. Behavior matches the [`ctx` CLI](https://github.com/RiddhiKatarki/ctx) exactly.

## License

[MIT](LICENSE)
