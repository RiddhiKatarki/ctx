import * as vscode from 'vscode';
import * as path from 'path';
import * as fs from 'fs';
import type { CtxClient, CtxError } from '../binary/client';
import type { ExportResult, StreamEvent } from '../types';
import { Settings, SecretStore } from '../config/settings';
import { reportError, reportSuccess } from '../ui/notifications';

const PROMPT_SOURCES = [
  { label: 'Auto-detect', value: 'auto', description: 'Pick the freshest provider' },
  { label: 'File', value: 'file', description: 'Load prompts from a JSON file' },
  { label: 'Claude Code', value: 'claudecode', description: '~/.claude/projects/*/*.jsonl' },
  { label: 'OpenCode', value: 'opencode', description: '~/.local/state/opencode/prompt-history.jsonl' },
  { label: 'Cursor', value: 'cursor', description: 'Cursor SQLite state.vscdb' },
  { label: 'Aider', value: 'aider', description: '~/.aider.chat.history[.md]' },
  { label: 'None (empty)', value: 'mock', description: 'Skip prompt collection' },
] as const;

const SUMMARY_PROVIDERS = [
  { label: 'Template (local)', value: 'template', description: 'Offline metadata-based summary' },
  { label: 'OpenAI-compatible', value: 'openai', description: 'Send to OpenAI / Venice / Ollama / vLLM' },
] as const;

export function makeExportCommand(
  getClient: () => Promise<CtxClient>,
  secrets: SecretStore,
) {
  return async (input?: unknown): Promise<void> => {
    const settings = Settings.create();

    // Resolve workspace folder.
    const folder = resolveWorkspaceFolder(input);
    if (!folder) {
      void vscode.window.showErrorMessage(
        'ctx export needs an open workspace folder.',
      );
      return;
    }

    // Pre-flight: ctx export requires a git repository. Fail fast with
    // an actionable message instead of making the user walk through the
    // whole QuickPick flow and then hit an opaque error.
    const gitSubfolder = findGitSubfolder(folder.uri.fsPath);
    if (!isGitRepo(folder.uri.fsPath)) {
      const buttons = gitSubfolder
        ? [`Open ${path.basename(gitSubfolder)}`]
        : ['Open Folder...'];
      const msg = gitSubfolder
        ? `ctx export needs a git repository. "${folder.name}" isn't one, but a git repo was found at "${path.basename(gitSubfolder)}".`
        : `ctx export needs a git repository. "${folder.name}" is not a git repo.`;
      const action = await vscode.window.showErrorMessage(msg, ...buttons);
      if (gitSubfolder && action === `Open ${path.basename(gitSubfolder)}`) {
        await vscode.commands.executeCommand('vscode.openFolder', vscode.Uri.file(gitSubfolder), false);
      } else if (action === 'Open Folder...') {
        await vscode.commands.executeCommand('workbench.action.files.openFolder');
      }
      return;
    }

    // Walk the user through the configuration QuickPick flow.
    const opts = await collectExportOptions(folder, settings, secrets);
    if (!opts) {
      return; // user cancelled
    }

    // Run export with a progress notification driven by stream events.
    try {
      const result = await runWithProgress(opts.args, opts.cwd, getClient, (event, progress) => {
        const msg = humanizeEvent(event);
        if (msg) {
          progress.report({ message: msg });
        }
      });
      reportSuccess(
        `Exported ${result.file_count} file(s) to ${path.basename(result.path)} (${formatBytes(result.bundle_size)})`,
        settings,
      );
      // Reveal the bundle in the explorer so the user sees it.
      await revealInExplorer(result.path);
      // Refresh the tree so the new bundle shows up.
      await vscode.commands.executeCommand('ctx.list.refresh');
    } catch (err) {
      reportError(err, settings);
    }
  };
}

interface ResolvedExport {
  args: string[];
  cwd: string;
}

async function collectExportOptions(
  folder: vscode.WorkspaceFolder,
  settings: Settings,
  secrets: SecretStore,
): Promise<ResolvedExport | undefined> {
  // 1. Prompt source.
  const sourcePick = await vscode.window.showQuickPick(PROMPT_SOURCES, {
    title: 'ctx export — prompt source',
    placeHolder: 'Where should prompts come from?',
  });
  if (!sourcePick) return undefined;

  const args: string[] = ['export'];

  if (sourcePick.value === 'file') {
    const picked = await vscode.window.showOpenDialog({
      canSelectMany: false,
      filters: { 'Prompts JSON': ['json'] },
      title: 'Select a prompts.json file',
      defaultUri: folder.uri,
    });
    if (!picked?.[0]) return undefined;
    args.push('--prompts', picked[0].fsPath, '--prompts-source', 'file');
  } else {
    args.push('--prompts-source', sourcePick.value);
  }

  // 2. Summary provider.
  const summaryPick = await vscode.window.showQuickPick(SUMMARY_PROVIDERS, {
    title: 'ctx export — summary provider',
    placeHolder: `Current default: ${settings.summaryProvider}`,
  });
  const providerValue = summaryPick?.value ?? settings.summaryProvider;
  args.push('--summary-provider', providerValue);

  if (providerValue === 'openai') {
    // Let the user override the endpoint and model per-export. Defaults
    // come from settings so the common case (OpenAI) needs no typing.
    const baseUrl = await vscode.window.showInputBox({
      title: 'ctx export — API base URL',
      prompt: 'OpenAI-compatible endpoint (appends /chat/completions)',
      value: settings.openaiBaseUrl,
      placeHolder: 'https://api.openai.com/v1',
      ignoreFocusOut: true,
      validateInput: (v) => (v.trim().length === 0 ? 'URL cannot be empty' : undefined),
    });
    if (baseUrl === undefined) return undefined;

    const model = await vscode.window.showInputBox({
      title: 'ctx export — model name',
      prompt: 'Model to use for summary generation',
      value: settings.openaiModel,
      placeHolder: 'gpt-4o, glm-5.2, llama3, etc.',
      ignoreFocusOut: true,
      validateInput: (v) => (v.trim().length === 0 ? 'Model cannot be empty' : undefined),
    });
    if (model === undefined) return undefined;

    const apiKey = await secrets.ensureApiKey();
    if (!apiKey) {
      void vscode.window.showErrorMessage('ctx export needs an API key for the openai provider.');
      return undefined;
    }
    args.push('--api-key', apiKey, '--api-base-url', baseUrl.trim(), '--model', model.trim());
  }

  // 3. Toggles.
  const toggles = [
    { label: 'Scan & redact secrets', picked: settings.secretScanEnabled, flag: 'scan' },
    { label: 'Embed file contents (self-contained)', picked: settings.includeContents, flag: 'contents' },
  ];
  const togglePicks = await vscode.window.showQuickPick(toggles, {
    title: 'ctx export — options',
    canPickMany: true,
    placeHolder: 'Space to toggle, Enter to accept',
  });
  // showQuickPick returns the picked items when canPickMany is true.
  const wantScan = togglePicks?.some((t) => t.flag === 'scan') ?? settings.secretScanEnabled;
  const wantContents = togglePicks?.some((t) => t.flag === 'contents') ?? settings.includeContents;

  if (!wantScan) {
    args.push('--no-secret-scan');
  }
  if (wantContents) {
    args.push('--include-contents', '--contents-threshold', String(settings.contentsThreshold));
  }

  // 4. Output path.
  const outputPath = path.join(folder.uri.fsPath, settings.defaultOutputName);
  args.push('--output', outputPath);

  return { args, cwd: folder.uri.fsPath };
}

async function runWithProgress(
  args: string[],
  cwd: string,
  getClient: () => Promise<CtxClient>,
  onEvent: (event: StreamEvent, progress: vscode.Progress<{ message?: string }>) => void,
): Promise<ExportResult> {
  const client = await getClient();
  return vscode.window.withProgress(
    {
      location: vscode.ProgressLocation.Notification,
      title: 'ctx export',
      cancellable: true,
    },
    async (progress, token) => {
      progress.report({ message: 'Starting…' });
      return client.runStream<ExportResult>(
        args,
        (event) => onEvent(event, progress),
        token,
        { cwd },
      );
    },
  );
}

function humanizeEvent(event: StreamEvent): string | undefined {
  switch (event.event) {
    case 'start': return 'Starting…';
    case 'git_detected': return 'Reading git repository…';
    case 'git_metadata': return 'Captured git metadata';
    case 'prompts_collected': return `Collected ${event.data?.count ?? 0} prompt(s)`;
    case 'files_collected': return `Found ${event.data?.count ?? 0} modified file(s)`;
    case 'diff_captured': return 'Captured uncommitted changes';
    case 'secrets_found': return `Redacted ${event.data?.count ?? 0} secret(s)`;
    case 'snapshot_built': return 'Built snapshot';
    case 'contents_read': return `Embedded ${event.data?.count ?? 0} file(s)`;
    case 'summary_start': return 'Generating summary…';
    case 'summary_complete': return 'Summary ready';
    case 'bundle_written': return 'Writing bundle…';
    default: return undefined;
  }
}

async function revealInExplorer(fsPath: string): Promise<void> {
  const uri = vscode.Uri.file(fsPath);
  await vscode.commands.executeCommand('revealInExplorer', uri);
}

function resolveWorkspaceFolder(input?: unknown): vscode.WorkspaceFolder | undefined {
  // From view/title: input may be a WorkspaceFolder.
  if (input instanceof vscode.Uri) {
    return vscode.workspace.getWorkspaceFolder(input);
  }
  if (vscode.workspace.workspaceFolders && vscode.workspace.workspaceFolders.length > 0) {
    return vscode.workspace.workspaceFolders[0];
  }
  return undefined;
}

/**
 * Returns true iff `dir` is inside (or is) a git repository.
 * Matches the Go CLI's notion of a repo: walks up parents looking
 * for a `.git` entry. We use fs rather than spawning `git` because
 * it's an order of magnitude faster and doesn't depend on git
 * being on PATH (the binary will check that itself later).
 */
function isGitRepo(dir: string): boolean {
  let current = path.resolve(dir);
  for (;;) {
    if (fs.existsSync(path.join(current, '.git'))) {
      return true;
    }
    const parent = path.dirname(current);
    if (parent === current) {
      return false;
    }
    current = parent;
  }
}

/**
 * Looks for an immediate child of `root` that is a git repo. Used to
 * offer a one-click "switch to the right folder" action when the user
 * has opened a parent folder (the common case: opening ~/code when
 * they meant ~/code/my-project).
 *
 * Only scans one level deep to avoid surprises — if there are multiple
 * git repos under the workspace, the user should pick manually.
 */
function findGitSubfolder(root: string): string | undefined {
  let entries: string[];
  try {
    entries = fs.readdirSync(root);
  } catch {
    return undefined;
  }
  const candidates: string[] = [];
  for (const entry of entries) {
    const full = path.join(root, entry);
    let stat;
    try {
      stat = fs.statSync(full);
    } catch {
      continue;
    }
    if (stat.isDirectory() && !entry.startsWith('.') && entry !== 'node_modules') {
      if (fs.existsSync(path.join(full, '.git'))) {
        candidates.push(full);
      }
    }
  }
  // Return the sole candidate; if there are several the user has to
  // pick manually because we can't guess which they meant.
  return candidates.length === 1 ? candidates[0] : undefined;
}

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / 1024 / 1024).toFixed(1)} MB`;
}

// Suppress unused-import warning for CtxError; it's used in type position
// by callers via the public error surface.
export type { CtxError };
