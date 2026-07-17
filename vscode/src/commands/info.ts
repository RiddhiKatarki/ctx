import * as vscode from 'vscode';
import * as path from 'path';
import * as os from 'os';
import * as fs from 'fs';
import type { CtxClient } from '../binary/client';
import type { InfoResult } from '../types';
import { log, logError, getOutputChannel } from '../util/logger';

export function makeInfoCommand(getClient: () => Promise<CtxClient>) {
  return async (input?: unknown): Promise<void> => {
    const bundlePath = await resolveBundleUri(input);
    if (!bundlePath) {
      log('info: no bundle path resolved, exiting');
      return;
    }

    log(`info: starting for ${bundlePath}`);

    let result: InfoResult;
    try {
      const client = await getClient();
      log(`info: running ctx info --json`);
      result = await client.runJSON<InfoResult>(['info', bundlePath]);
      log(`info: got result, valid=${result.valid}, file_count=${result.file_count}`);
    } catch (err) {
      logError('info: command failed', err);
      void vscode.window.showErrorMessage(
        `ctx info failed: ${err instanceof Error ? err.message : String(err)}`,
        'Open Logs',
      ).then((action) => {
        if (action === 'Open Logs') {
          getOutputChannel().show();
        }
      });
      return;
    }

    try {
      await renderInfo(result);
      log('info: rendered');
    } catch (err) {
      logError('info: render failed', err);
      void vscode.window.showErrorMessage(
        `ctx info render failed: ${err instanceof Error ? err.message : String(err)}`,
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
    title: 'Select a .ctx bundle',
  });
  return picked?.[0]?.fsPath;
}

/**
 * Renders bundle metadata as Markdown in a temp file, opened in VS
 * Code's built-in Markdown preview. Same rationale as inspect.ts:
 * reliable across desktop, code-server, and web.
 */
async function renderInfo(result: InfoResult): Promise<void> {
  const md = renderMarkdown(result);

  const tempDir = path.join(os.tmpdir(), 'ctx-info');
  fs.mkdirSync(tempDir, { recursive: true });
  const safeName = basename(result.path).replace(/[^a-z0-9.-]+/gi, '_');
  const mdPath = path.join(tempDir, `${safeName}.md`);
  fs.writeFileSync(mdPath, md, 'utf8');

  const uri = vscode.Uri.file(mdPath);
  const doc = await vscode.workspace.openTextDocument(uri);
  await vscode.window.showTextDocument(doc, { preview: false, viewColumn: vscode.ViewColumn.One });
  await vscode.commands.executeCommand('markdown.showPreviewToSide', uri);
}

function renderMarkdown(result: InfoResult): string {
  const lines: string[] = [];
  lines.push(`# Bundle info`);
  lines.push('');

  lines.push('| Field | Value |');
  lines.push('| --- | --- |');
  lines.push(`| Path | \`${result.path}\` |`);
  lines.push(`| Size | ${formatBytes(result.size)} |`);
  lines.push(`| Valid | ${result.valid ? 'yes' : 'no'} |`);
  lines.push(`| Files | ${result.file_count} |`);
  lines.push(`| Has diff | ${result.has_diff ? `yes (${formatBytes(result.diff_size)})` : 'no'} |`);
  lines.push(`| Summary length | ${result.summary_length} chars |`);
  lines.push('');

  lines.push('## Manifest');
  lines.push('');
  lines.push('```json');
  lines.push(JSON.stringify(result.manifest, null, 2));
  lines.push('```');
  lines.push('');

  lines.push('## Metadata');
  lines.push('');
  lines.push('```json');
  lines.push(JSON.stringify(result.metadata, null, 2));
  lines.push('```');
  lines.push('');

  if (result.git && Object.keys(result.git).length > 0) {
    lines.push('## Git');
    lines.push('');
    lines.push('```json');
    lines.push(JSON.stringify(result.git, null, 2));
    lines.push('```');
    lines.push('');
  }

  if (result.files.length > 0) {
    lines.push(`## Files (${result.files.length})`);
    lines.push('');
    for (const f of result.files) {
      lines.push(`- \`${f}\``);
    }
    lines.push('');
  }

  return lines.join('\n');
}

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / 1024 / 1024).toFixed(1)} MB`;
}

function basename(p: string): string {
  const norm = p.replace(/\\/g, '/');
  const parts = norm.split('/');
  return parts[parts.length - 1] || p;
}
