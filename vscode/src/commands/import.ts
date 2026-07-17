import * as vscode from 'vscode';
import * as path from 'path';
import * as os from 'os';
import type { CtxClient } from '../binary/client';
import type { ImportResult, StreamEvent } from '../types';
import { Settings } from '../config/settings';
import { reportError, reportSuccess } from '../ui/notifications';
import { log } from '../util/logger';

export function makeImportCommand(getClient: () => Promise<CtxClient>) {
  return async (input?: unknown): Promise<void> => {
    const settings = Settings.create();

    const bundlePath = await resolveBundleUri(input);
    if (!bundlePath) return;

    // Ask whether to extract contents.
    const action = await vscode.window.showInformationMessage(
      `Import ${path.basename(bundlePath)}?`,
      'Validate only',
      'Extract to folder',
      'Cancel',
    );
    if (action === 'Cancel' || !action) return;

    const args: string[] = ['import', bundlePath];
    let extractTo: string | undefined;
    if (action === 'Extract to folder') {
      const defaultOut = path.join(path.dirname(bundlePath), settings.defaultOutdir);
      extractTo = await pickOutdir(defaultOut);
      if (!extractTo) return;
      args.push('--outdir', extractTo);
    }

    try {
      const result = await runImportWithProgress(args, getClient);
      const summary = formatSummary(result);
      reportSuccess(summary, settings);

      if (extractTo) {
        const patchAction = await vscode.window.showInformationMessage(
          `${summary} Extracted to ${path.basename(extractTo)}.`,
          'Apply patch',
        );
        if (patchAction === 'Apply patch') {
          await applyPatch(path.join(extractTo, 'patch.diff'));
        }
      }
    } catch (err) {
      reportError(err, settings);
    }
  };
}

/**
 * ctx.applyPatch command. Unpacks patch.diff from a .ctx bundle into
 * a temp dir, then runs `git apply` in the integrated terminal.
 */
export function makeApplyPatchCommand(getClient: () => Promise<CtxClient>) {
  return async (input?: unknown): Promise<void> => {
    const bundlePath = await resolveBundleUri(input);
    if (!bundlePath) return;

    const settings = Settings.create();
    const tempOut = path.join(os.tmpdir(), `ctx-patch-${Date.now()}`);

    try {
      const client = await getClient();
      await client.runJSON(['import', bundlePath, '--outdir', tempOut]);
      await applyPatch(path.join(tempOut, 'patch.diff'));
    } catch (err) {
      reportError(err, settings);
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
    title: 'Select a .ctx bundle to import',
  });
  return picked?.[0]?.fsPath;
}

async function pickOutdir(defaultPath: string): Promise<string | undefined> {
  const picked = await vscode.window.showOpenDialog({
    canSelectMany: false,
    canSelectFiles: false,
    canSelectFolders: true,
    defaultUri: vscode.Uri.file(defaultPath),
    title: 'Choose a directory to extract the bundle into',
  });
  return picked?.[0]?.fsPath;
}

async function runImportWithProgress(
  args: string[],
  getClient: () => Promise<CtxClient>,
): Promise<ImportResult> {
  const client = await getClient();
  return vscode.window.withProgress(
    {
      location: vscode.ProgressLocation.Notification,
      title: 'ctx import',
      cancellable: true,
    },
    (progress, token) => {
      progress.report({ message: 'Reading bundle…' });
      return client.runStream<ImportResult>(
        args,
        (event) => {
          const msg = humanizeImportEvent(event);
          if (msg) {
            progress.report({ message: msg });
          }
        },
        token,
      );
    },
  );
}

function humanizeImportEvent(event: StreamEvent): string | undefined {
  switch (event.event) {
    case 'start': return 'Reading bundle…';
    case 'extracted': return 'Unpacked archive';
    case 'manifest_validated': return 'Manifest validated';
    case 'extracted_to': return `Extracted to ${event.data?.path ?? ''}`;
    default: return undefined;
  }
}

function formatSummary(r: ImportResult): string {
  const parts = [
    `${r.project_name} (${r.branch})`,
    `${r.file_count} file(s)`,
  ];
  if (r.has_diff) {
    parts.push(`${r.diff_size}-byte diff`);
  }
  if (r.prompt_count > 0) {
    parts.push(`${r.prompt_count} prompt(s)`);
  }
  return parts.join(', ');
}

async function applyPatch(patchPath: string): Promise<void> {
  const terminal = vscode.window.createTerminal('ctx: apply patch');
  terminal.show();
  terminal.sendText(`git apply "${patchPath}"`);
  log(`sent 'git apply ${patchPath}' to terminal`);
}
