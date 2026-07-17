import * as vscode from 'vscode';
import * as path from 'path';
import * as os from 'os';
import * as fs from 'fs';
import type { CtxClient } from '../binary/client';
import type { InspectResult, Summary } from '../types';
import { log, logError, getOutputChannel } from '../util/logger';

const SECTION_LABELS: { key: keyof Summary; label: string }[] = [
  { key: 'current_objective', label: 'Current Objective' },
  { key: 'completed_work', label: 'Completed Work' },
  { key: 'remaining_tasks', label: 'Remaining Tasks' },
  { key: 'known_bugs', label: 'Known Bugs' },
  { key: 'architecture_decisions', label: 'Architecture Decisions' },
  { key: 'files_to_read_first', label: 'Files To Read First' },
  { key: 'previous_failed_approaches', label: 'Previous Failed Approaches' },
  { key: 'suggested_next_prompt', label: 'Suggested Next Prompt' },
  { key: 'estimated_reading_time', label: 'Estimated Reading Time' },
];

export function makeInspectCommand(getClient: () => Promise<CtxClient>) {
  return async (input?: unknown): Promise<void> => {
    const bundlePath = await resolveBundleUri(input);
    if (!bundlePath) {
      log('inspect: no bundle path resolved, exiting');
      return;
    }

    log(`inspect: starting for ${bundlePath}`);

    let result: InspectResult;
    try {
      const client = await getClient();
      log(`inspect: client obtained, running ctx inspect --json`);
      result = await client.runJSON<InspectResult>(['inspect', bundlePath]);
      log(`inspect: got result, valid=${result.valid}, sections=${Object.keys(result.summary_sections).length}`);
    } catch (err) {
      logError('inspect: command failed', err);
      void vscode.window.showErrorMessage(
        `ctx inspect failed: ${err instanceof Error ? err.message : String(err)}`,
        'Open Logs',
      ).then((action) => {
        if (action === 'Open Logs') {
          getOutputChannel().show();
        }
      });
      return;
    }

    try {
      await renderInspect(result, bundlePath);
      log('inspect: rendered');
    } catch (err) {
      logError('inspect: render failed', err);
      void vscode.window.showErrorMessage(
        `ctx inspect render failed: ${err instanceof Error ? err.message : String(err)}`,
        'Open Logs',
      ).then((action) => {
        if (action === 'Open Logs') {
          getOutputChannel().show();
        }
      });
    }
  };
}

async function resolveBundleUri(input?: unknown): Promise<string | undefined> {
  if (input instanceof vscode.Uri) {
    return input.fsPath;
  }
  if (typeof input === 'object' && input !== null && 'fsPath' in input) {
    return String((input as { fsPath: unknown }).fsPath);
  }
  const active = vscode.window.activeTextEditor?.document.uri;
  if (active && active.fsPath.endsWith('.ctx')) {
    return active.fsPath;
  }
  const picked = await vscode.window.showOpenDialog({
    canSelectMany: false,
    filters: { 'ctx bundle': ['ctx'] },
    title: 'Select a .ctx bundle to inspect',
  });
  return picked?.[0]?.fsPath;
}

/**
 * Renders the inspect result by writing a Markdown file to a temp
 * directory and opening it in VS Code's built-in Markdown preview.
 *
 * This is more reliable than a custom WebView across environments
 * (desktop VS Code, code-server, vscode.dev) because it uses the
 * host's battle-tested markdown renderer instead of a bespoke
 * HTML/CSS/JS bundle that may be blocked by CSP or iframe sandboxing.
 *
 * The Markdown source is also kept open in a text editor tab so users
 * can copy the raw summary, edit it, or save it elsewhere.
 */
async function renderInspect(result: InspectResult, bundlePath: string): Promise<void> {
  const title = bundleTitleFromMetadata(result, bundlePath);
  const md = renderMarkdown(result, title, bundlePath);

  // Write to a temp file in the OS temp dir so we never pollute the
  // user's workspace. The filename includes the bundle's basename so
  // multiple bundles can be inspected simultaneously without collision.
  const tempDir = path.join(os.tmpdir(), 'ctx-inspect');
  fs.mkdirSync(tempDir, { recursive: true });
  const safeName = basename(bundlePath).replace(/[^a-z0-9.-]+/gi, '_');
  const mdPath = path.join(tempDir, `${safeName}.md`);
  fs.writeFileSync(mdPath, md, 'utf8');

  const uri = vscode.Uri.file(mdPath);

  // Open the markdown source in an editor tab (non-preview so it stays).
  const doc = await vscode.workspace.openTextDocument(uri);
  await vscode.window.showTextDocument(doc, { preview: false, viewColumn: vscode.ViewColumn.One });

  // Open the rendered Markdown preview beside it.
  await vscode.commands.executeCommand('markdown.showPreviewToSide', uri);
}

/**
 * Renders the inspect result as GitHub-flavored Markdown. Mirrors the
 * 9 canonical section labels and the bundle's metadata header.
 */
function renderMarkdown(result: InspectResult, title: string, bundlePath: string): string {
  const lines: string[] = [];
  lines.push(`# ${title}`);
  lines.push('');
  lines.push(`> Bundle: \`${basename(bundlePath)}\` · ${result.files.length} file${result.files.length === 1 ? '' : 's'}${result.valid ? '' : ' · ⚠ invalid'}`);
  lines.push('');
  lines.push('---');
  lines.push('');

  for (const { key, label } of SECTION_LABELS) {
    const text = result.summary_sections[key] || '(not provided)';
    lines.push(`## ${label}`);
    lines.push('');
    lines.push(text);
    lines.push('');
  }

  if (result.files.length > 0) {
    lines.push('---');
    lines.push('');
    lines.push(`## Files (${result.files.length})`);
    lines.push('');
    for (const f of result.files) {
      lines.push(`- \`${f}\``);
    }
    lines.push('');
  }

  return lines.join('\n');
}

function bundleTitleFromMetadata(result: InspectResult, fallback: string): string {
  const meta = result.metadata as { project_name?: string; branch?: string };
  if (meta?.project_name) {
    return meta.project_name + (meta.branch ? ` (${meta.branch})` : '');
  }
  return basename(fallback);
}

function basename(p: string): string {
  const norm = p.replace(/\\/g, '/');
  const parts = norm.split('/');
  return parts[parts.length - 1] || p;
}
