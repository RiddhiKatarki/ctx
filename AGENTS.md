# AGENTS.md

Guidance for AI coding agents (Claude, Cursor, OpenCode, Copilot, etc.) working in this repo.

## Repo shape

Two coupled projects in one repo:

- **`/`** — `ctx` Go CLI. Module `github.com/RiddhiKatarki/ctx`. Go 1.25.0.
- **`/vscode/`** — VS Code extension that wraps the CLI. TypeScript, Node 20+.

The extension spawns prebuilt Go binaries at runtime (binary-wrap model). They share a release pipeline and version together.

## Commands

### Go CLI (run from repo root)

```bash
go test ./...                    # tests
go vet ./...                     # vet
gofmt -l .                       # format check (see gotcha below)
go build -o ctx ./cmd/ctx        # local binary
```

Cross-compile one binary for the extension's dev workflow:

```bash
go build -trimpath -ldflags "-s -w" -o vscode/bin/ctx-linux-amd64 ./cmd/ctx
```

### VS Code extension (run from `vscode/`)

```bash
npm test                         # compile + unit + integration
npm run test:unit                # vitest, no binary needed
npm run test:integration         # vitest, REQUIRES bin/ctx-linux-amd64
npm run compile                  # tsc --noEmit + esbuild bundle
npm run lint                     # eslint
npm run package                  # esbuild production bundle
npx vsce package --no-yarn       # build .vsix (use --pre-release for x.y.z-rc.N)
```

First-time extension setup: `cd vscode && npm install`, then build a local binary into `vscode/bin/ctx-linux-amd64` so integration tests can run.

## Architecture

### Go CLI

- `cmd/ctx/main.go` — Cobra entrypoint. Single file. All flags + subcommands.
- `internal/` — implementation packages, each independently testable:
  - `archive` — ZIP (.ctx) create/extract/peek
  - `bundle` — in-memory bundle model + (de)serialization
  - `clierr` — **stable exit-code contract** (see below)
  - `discovery` — `list` + `info` bundle scanning
  - `export` — export orchestration (largest package)
  - `git` — `GitProvider` interface + CLI impl
  - `import` — import orchestration
  - `inspect` — inspect orchestration
  - `providers` — `PromptProvider` + Claude/OpenCode/Cursor/Aider/File/Mock
  - `reporter` — Human/JSON/Stream output abstraction
  - `schema` — format constants, filenames, validation
  - `secretscan` — regex + entropy secret detection, `.ctxignore`
  - `summary` — `SummaryProvider` + template + OpenAI
- `pkg/types/` — shared domain types (zero internal deps).

### VS Code extension

- `src/binary/` — spawn the Go binary, parse JSON/NDJSON, exit-code mapping.
- `src/commands/` — one file per command. Each owns its full flow including error UX.
- `src/providers/` — TreeDataProvider, FileDecorationProvider.
- `src/config/` — typed Settings wrapper + SecretStore for API key.
- `src/ui/` — notification routing (warning vs error vs invalid_bundle).

### Stable contract (do not break)

Exit codes from `internal/clierr/clierr.go` — tooling relies on these:

| Exit | Code             |
|------|------------------|
| 0    | success          |
| 1    | `user_error`     |
| 2    | `system_error`   |
| 3    | `invalid_bundle` |

JSON error envelope on stderr (when `--json`):

```json
{"error": {"code": "user_error", "message": "...", "cause": "..."}}
```

### Bundle format

`.ctx` is a ZIP of 7 canonical files: `manifest.json`, `metadata.json`, `git.json`, `summary.md`, `prompts.json`, `files.json`, `patch.diff`. Optional `contents/` prefix for self-contained V2 bundles. Schema version lives in `internal/schema/schema.go:BundleVersion`.

### Cursor prompt provider

`internal/providers/cursor.go` reads SQLite (`state.vscdb`) via `modernc.org/sqlite` (pure Go, no cgo). Two schemas:
- Modern (Cursor 3.x): `cursorDiskKV` with `composerData:<id>` / `bubbleId:<cid>:<bid>` keys
- Legacy (≤0.40): `ItemTable[workbench.panel.aichat.view.aichat.chatdata]`

Per-project filtering via `glass.localAgentProjects.v1` + `glass.localAgentProjectMembership.v1`.

## Conventions

- **Commits**: lowercase conventional style (`feat:`, `fix:`, `docs:`, `ci:`). See `git log` for examples.
- **No emojis** in code, files, or commit messages.
- **No comments** unless the user explicitly asks.
- **Tests live next to code** (Go: `foo_test.go`; TS: `test/unit/*.test.ts` + `test/integration/*.test.ts`).
- **API keys** go in VS Code SecretStorage, never `settings.json`, never logs, never git.
- **Inspect/Info render via Markdown preview**, not custom WebViews. WebViews render blank in code-server's iframe sandbox.
- **README/code lag**: the codebase predates gofmt enforcement. Don't reformat files you didn't touch.

## Gotchas

- **gofmt**: 14 pre-existing files fail `gofmt -l .` on `main`. CI scopes the check to `git diff origin/main...HEAD -- '*.go'`. Run `gofmt -w` only on files you modify.
- **Go version**: 1.25.0 (bumped from 1.22.5 because `modernc.org/sqlite` requires it). Don't downgrade.
- **Cross-compile**: all 6 GOOS/GOARCH combos build on `ubuntu-latest` with `CGO_ENABLED=0`. Do not switch to `macos-*` or `windows-*` runners — they're slower and scarcer.
- **`export` ignores positional `cwd`**: the Go CLI uses `os.Getwd()`. The extension's `CtxClient` accepts `RunOptions.cwd` threaded through `spawn()`.
- **`list`/`info`/`inspect` don't stream**: they only support `--json`. Don't pass `--stream` to them.
- **First-time publish**: needs a Marketplace PAT (`VSCE_PAT` secret) for automated `vsce publish`. Without it, the `.vsix` is still attached to the GitHub Release for manual upload via <https://marketplace.visualstudio.com/manage>.

## Release

Tags matching `v*` trigger `.github/workflows/release.yml`:

1. Build 6 binaries (ubuntu-latest matrix, ~40s each)
2. Download all → stage into `vscode/bin/`
3. `npm ci && npm run package && npx vsce package`
4. Attach `.vsix` + binaries to GitHub Release
5. `vsce publish` only runs if `VSCE_PAT` is set (separate `check-pat` guard job — `secrets` context can't be referenced in job-level `if:`)

Bump version in `vscode/package.json`, the badge in the same file, and add a `CHANGELOG.md` entry before tagging. Keep Go CLI version (`cmd/ctx/main.go:version`) and extension version in lockstep across a release tag.
