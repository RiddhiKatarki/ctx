import * as vscode from 'vscode';
import type { CtxClient } from '../binary/client';
import type { InfoResult } from '../types';

/**
 * ctx.info command. Resolves a .ctx file from either:
 *   - the command argument (explorer/context menu click), or
 *   - the active editor's document, or
 *   - a user file picker prompt.
 *
 * Then runs `ctx info --json` and renders the result in a WebView panel.
 */
export function makeInfoCommand(getClient: () => Promise<CtxClient>) {
  return async (input?: unknown): Promise<void> => {
    const path = await resolveBundleUri(input);
    if (!path) {
      return; // user cancelled
    }

    const client = await getClient();
    const result = await client.runJSON<InfoResult>(['info', path]);
    await renderInfoPanel(result);
  };
}

async function resolveBundleUri(input?: unknown): Promise<string | undefined> {
  // From explorer/context menu: input is a Uri or { fsPath }.
  if (input instanceof vscode.Uri) {
    return input.fsPath;
  }
  if (typeof input === 'object' && input !== null && 'fsPath' in input) {
    return String((input as { fsPath: unknown }).fsPath);
  }

  // From editor/title: the active document.
  const active = vscode.window.activeTextEditor?.document.uri;
  if (active && active.fsPath.endsWith('.ctx')) {
    return active.fsPath;
  }

  // Fallback: file picker.
  const picked = await vscode.window.showOpenDialog({
    canSelectMany: false,
    filters: { 'ctx bundle': ['ctx'] },
    title: 'Select a .ctx bundle',
  });
  return picked?.[0]?.fsPath;
}

async function renderInfoPanel(result: InfoResult): Promise<void> {
  const panel = vscode.window.createWebviewPanel(
    'ctx.info',
    `ctx: ${basename(result.path)}`,
    vscode.ViewColumn.Active,
    { enableFindWidget: true },
  );
  panel.webview.html = renderHtml(result);
}

function renderHtml(result: InfoResult): string {
  const rows: string[] = [];
  rows.push(`<tr><th>Path</th><td><code>${escape(result.path)}</code></td></tr>`);
  rows.push(`<tr><th>Size</th><td>${formatBytes(result.size)}</td></tr>`);
  rows.push(`<tr><th>Valid</th><td>${result.valid ? '✓ yes' : '✗ no'}</td></tr>`);
  rows.push(`<tr><th>Files</th><td>${result.file_count}</td></tr>`);
  rows.push(`<tr><th>Has diff</th><td>${result.has_diff ? 'yes' : 'no'} (${formatBytes(result.diff_size)})</td></tr>`);
  rows.push(`<tr><th>Summary length</th><td>${result.summary_length} chars</td></tr>`);

  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>ctx info: ${escape(basename(result.path))}</title>
  <style>
    body { font-family: var(--vscode-font-family, sans-serif); color: var(--vscode-foreground); padding: 1rem; }
    h1 { font-size: 1.1rem; margin-top: 0; }
    table { border-collapse: collapse; margin: 1rem 0; }
    th { text-align: left; padding: 0.25rem 1rem 0.25rem 0; color: var(--vscode-descriptionForeground); font-weight: normal; vertical-align: top; }
    td { padding: 0.25rem 0; }
    pre { background: var(--vscode-textCodeBlock-background); padding: 0.5rem; border-radius: 3px; overflow: auto; max-height: 300px; }
    code { font-family: var(--vscode-editor-font-family, monospace); }
    .tabs button { background: none; border: 1px solid var(--vscode-input-border, #ccc); color: var(--vscode-foreground); padding: 0.25rem 0.5rem; cursor: pointer; margin-right: 0.25rem; }
    .tabs button.active { background: var(--vscode-button-background); color: var(--vscode-button-foreground); border-color: var(--vscode-button-background); }
  </style>
</head>
<body>
  <h1>Bundle info</h1>
  <table>${rows.join('\n')}</table>

  <h2>Manifest</h2>
  <pre>${escape(JSON.stringify(result.manifest, null, 2))}</pre>

  <h2>Metadata</h2>
  <pre>${escape(JSON.stringify(result.metadata, null, 2))}</pre>

  ${result.git && Object.keys(result.git).length > 0 ? `<h2>Git</h2><pre>${escape(JSON.stringify(result.git, null, 2))}</pre>` : ''}

  ${result.files.length > 0 ? `<h2>Files (${result.files.length})</h2><ul>${result.files.map((f) => `<li><code>${escape(f)}</code></li>`).join('\n')}</ul>` : ''}
</body>
</html>`;
}

function escape(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
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
