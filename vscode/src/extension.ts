import * as vscode from 'vscode';
import { COMMANDS, EXTENSION_ID } from './constants';
import { log } from './util/logger';
import { getClient, hasBinary } from './binary/factory';
import { SecretStore } from './config/settings';
import { makeInfoCommand } from './commands/info';
import { makeInspectCommand } from './commands/inspect';
import { makeExportCommand } from './commands/export';
import { makeImportCommand, makeApplyPatchCommand } from './commands/import';
import { makeUpdateApiKeyCommand } from './commands/updateApiKey';
import { BundleTreeProvider } from './providers/bundleTreeProvider';
import { BundleDecorationProvider } from './providers/fileDecorationProvider';

export async function activate(context: vscode.ExtensionContext): Promise<ExtensionApi> {
  log(`activating ${EXTENSION_ID}`);

  const secrets = new SecretStore(context.secrets);

  // Lazy client accessor — closes over the extension path.
  const getClientFor = async () => getClient(context.extensionPath);

  // Register commands. Each command handles its own error UX so
  // activate() stays linear.
  const disposables: vscode.Disposable[] = [];

  disposables.push(
    vscode.commands.registerCommand(COMMANDS.exportCtx, makeExportCommand(getClientFor, secrets)),
  );
  disposables.push(
    vscode.commands.registerCommand(COMMANDS.importCtx, makeImportCommand(getClientFor)),
  );
  disposables.push(
    vscode.commands.registerCommand(COMMANDS.inspect, makeInspectCommand(getClientFor)),
  );
  disposables.push(
    vscode.commands.registerCommand(COMMANDS.info, makeInfoCommand(getClientFor)),
  );
  disposables.push(
    vscode.commands.registerCommand(COMMANDS.applyPatch, makeApplyPatchCommand(getClientFor)),
  );
  disposables.push(
    vscode.commands.registerCommand('ctx.updateApiKey', makeUpdateApiKeyCommand(secrets)),
  );

  // Tree view — refreshes when .ctx files change in the workspace.
  const treeProvider = new BundleTreeProvider(getClientFor);
  disposables.push(
    vscode.window.registerTreeDataProvider('ctx.bundles', treeProvider),
  );
  disposables.push(
    vscode.commands.registerCommand(COMMANDS.listRefresh, () => treeProvider.refresh()),
  );

  // File decoration — subtle badge for .ctx files in the explorer.
  const decorationProvider = new BundleDecorationProvider();
  disposables.push(
    vscode.window.registerFileDecorationProvider(decorationProvider),
  );

  // Auto-refresh on workspace file changes (.ctx files added/removed).
  const watcher = vscode.workspace.createFileSystemWatcher('**/*.ctx');
  disposables.push(watcher);
  disposables.push(watcher.onDidCreate(() => { treeProvider.refresh(); decorationProvider.refresh(); }));
  disposables.push(watcher.onDidDelete(() => { treeProvider.refresh(); decorationProvider.refresh(); }));
  disposables.push(watcher.onDidChange(() => { treeProvider.refresh(); decorationProvider.refresh(); }));

  context.subscriptions.push(...disposables);

  // Surface a one-time warning if the binary isn't bundled for this
  // platform (e.g. marketplace install on unsupported OS).
  if (!hasBinary(context.extensionPath)) {
    void vscode.window.showWarningMessage(
      `ctx: no prebuilt binary for ${process.platform}/${process.arch}. ` +
        `Commands will fail until you install a compatible binary.`,
    );
  }

  log(`activated ${EXTENSION_ID}`);
  return { extensionPath: context.extensionPath };
}

export function deactivate(): void {
  log(`deactivating ${EXTENSION_ID}`);
}

export interface ExtensionApi {
  extensionPath: string;
}
