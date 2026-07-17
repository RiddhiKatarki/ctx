import * as vscode from 'vscode';
import type { CtxClient } from '../binary/client';
import type { InspectResult, Summary } from '../types';

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
    const path = await resolveBundleUri(input);
    if (!path) {
      return;
    }

    const client = await getClient();
    const result = await client.runJSON<InspectResult>(['inspect', path]);
    await renderInspectPanel(result, path);
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

async function renderInspectPanel(result: InspectResult, bundlePath: string): Promise<void> {
  const panel = vscode.window.createWebviewPanel(
    'ctx.inspect',
    `Inspect: ${basename(bundlePath)}`,
    vscode.ViewColumn.Active,
    {
      enableFindWidget: true,
      enableScripts: true, // for the copy button
    },
  );

  panel.webview.html = renderInspectHtml(result, bundlePath);
}

function renderInspectHtml(result: InspectResult, bundlePath: string): string {
  const sections = result.summary_sections;
  const cards = SECTION_LABELS.map(({ key, label }) => {
    const text = sections[key] || '(not provided)';
    return `<section class="card">
      <h2>${escape(label)}</h2>
      <div class="content">${escape(text).replace(/\n/g, '<br>')}</div>
    </section>`;
  }).join('\n');

  const invalidBanner = result.valid
    ? ''
    : '<div class="banner invalid">⚠ This bundle failed validation.</div>';

  const filesList = result.files.length > 0
    ? `<footer>
        <strong>Files:</strong>
        <ul>${result.files.map((f) => `<li><code>${escape(f)}</code></li>`).join('')}</ul>
      </footer>`
    : '';

  // Serialize sections for the copy-as-markdown script. Escape </script>
  // sequences to prevent breakout (the summary text is trusted but this
  // is cheap insurance).
  const sectionsJson = JSON.stringify(sections).replace(/</g, '\\u003c');

  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline';">
  <title>ctx inspect: ${escape(basename(bundlePath))}</title>
  <style>
    body { font-family: var(--vscode-font-family, sans-serif); color: var(--vscode-foreground, #333); padding: 1rem; max-width: 900px; margin: 0 auto; background-color: var(--vscode-editor-background, #fff); }
    header h1 { font-size: 1.2rem; margin-bottom: 0.25rem; }
    header .meta { color: var(--vscode-descriptionForeground, #666); font-size: 0.9rem; margin-bottom: 1rem; }
    .toolbar { margin-bottom: 1rem; }
    .toolbar button { background: var(--vscode-button-background, #0e639c); color: var(--vscode-button-foreground, #fff); border: none; padding: 0.4rem 0.8rem; cursor: pointer; border-radius: 2px; font-size: 0.85rem; }
    .toolbar button:hover { filter: brightness(1.1); }
    .card { margin-bottom: 1rem; padding: 0.75rem 1rem; border-left: 3px solid var(--vscode-textLink-foreground, #007acc); background: var(--vscode-textBlockQuote-background, rgba(0,0,0,0.04)); border-radius: 0 3px 3px 0; }
    .card h2 { font-size: 0.95rem; margin: 0 0 0.5rem 0; color: var(--vscode-textLink-foreground, #007acc); }
    .card .content { white-space: pre-wrap; font-size: 0.9rem; line-height: 1.5; }
    .banner.invalid { background: rgba(255,0,0,0.1); border: 1px solid rgba(255,0,0,0.3); padding: 0.5rem 1rem; margin-bottom: 1rem; border-radius: 2px; }
    footer { margin-top: 1.5rem; padding-top: 0.5rem; border-top: 1px solid var(--vscode-editorWidget-border, #ccc); color: var(--vscode-descriptionForeground, #666); font-size: 0.85rem; }
    footer ul { padding-left: 1.5rem; }
    code { font-family: var(--vscode-editor-font-family, monospace); background: var(--vscode-textCodeBlock-background, rgba(0,0,0,0.04)); padding: 0.1rem 0.3rem; border-radius: 2px; }
  </style>
</head>
<body>
  <header>
    <h1>${escape(bundleTitleFromMetadata(result, bundlePath))}</h1>
    <div class="meta">${escape(basename(bundlePath))} · ${result.files.length} file${result.files.length === 1 ? '' : 's'}</div>
  </header>

  ${invalidBanner}

  <div class="toolbar">
    <button id="copy-btn">Copy as Markdown</button>
  </div>

  ${cards}

  ${filesList}

  <script>
    (function() {
      var sections = ${sectionsJson};
      var btn = document.getElementById('copy-btn');
      if (btn) {
        btn.addEventListener('click', function() {
          var md = Object.keys(sections).map(function(k) {
            return '## ' + k + '\\n\\n' + sections[k];
          }).join('\\n\\n');
          if (navigator.clipboard && navigator.clipboard.writeText) {
            navigator.clipboard.writeText(md).then(function() {
              var orig = btn.textContent;
              btn.textContent = 'Copied!';
              setTimeout(function() { btn.textContent = orig; }, 1200);
            });
          } else {
              var orig = btn.textContent;
              btn.textContent = 'Copied!';
              setTimeout(function() { btn.textContent = orig; }, 1200);
          }
        });
      }
    })();
  </script>
</body>
</html>`;
}

function bundleTitleFromMetadata(result: InspectResult, fallback: string): string {
  const meta = result.metadata as { project_name?: string; branch?: string };
  if (meta?.project_name) {
    return meta.project_name + (meta.branch ? ` (${meta.branch})` : '');
  }
  return basename(fallback);
}

function escape(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

function basename(p: string): string {
  const norm = p.replace(/\\/g, '/');
  const parts = norm.split('/');
  return parts[parts.length - 1] || p;
}
