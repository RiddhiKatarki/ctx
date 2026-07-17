import * as vscode from 'vscode';
import { COMMANDS, EXTENSION_ID, MIN_BINARY_VERSION } from './constants';
import { log } from './util/logger';

/**
 * Called by VS Code when the extension activates.
 *
 * Activation triggers:
 *  - onStartupFinished (background — doesn't slow window load)
 *  - any of the ctx.* commands
 *  - the ctx.bundles view
 *
 * We activate eagerly enough to register the TreeView and ensure the
 * bundled binary is executable, but defer the heavier version check
 * until first command use.
 */
export async function activate(context: vscode.ExtensionContext): Promise<ExtensionApi> {
  log(`activating ${EXTENSION_ID}`);

  // Register every command declared in package.json. Phase C uses a
  // uniform placeholder; Phase E replaces handlers one by one.
  const registrations: vscode.Disposable[] = [];

  for (const cmdId of Object.values(COMMANDS)) {
    registrations.push(
      vscode.commands.registerCommand(cmdId, makePlaceholder(cmdId)),
    );
  }

  // Placeholder TreeView so the activity-bar icon shows something useful.
  const treeProvider = new PlaceholderTreeProvider();
  registrations.push(vscode.window.registerTreeDataProvider('ctx.bundles', treeProvider));

  context.subscriptions.push(...registrations);

  log(`activated ${EXTENSION_ID}; ${registrations.length} contributions registered`);

  return {
    extensionPath: context.extensionPath,
    minBinaryVersion: MIN_BINARY_VERSION,
  };
}

/**
 * Called when the extension is deactivated or VS Code is closing.
 * Nothing to dispose beyond context.subscriptions, which VS Code
 * handles automatically.
 */
export function deactivate(): void {
  log(`deactivating ${EXTENSION_ID}`);
}

export interface ExtensionApi {
  extensionPath: string;
  minBinaryVersion: string;
}

function makePlaceholder(cmdId: string): (...args: unknown[]) => void {
  return (...args) => {
    const argSummary = args.length === 0 ? '(no args)' : JSON.stringify(args);
    const msg = `[ctx] "${cmdId}" not yet implemented. Args: ${argSummary}`;
    log(msg);
    void vscode.window.showInformationMessage(msg);
  };
}

class PlaceholderTreeProvider implements vscode.TreeDataProvider<PlaceholderItem> {
  private readonly _onDidChange = new vscode.EventEmitter<PlaceholderItem | undefined>();
  readonly onDidChangeTreeData = this._onDidChange.event;

  getTreeItem(element: PlaceholderItem): vscode.TreeItem {
    return element;
  }

  getChildren(element?: PlaceholderItem): PlaceholderItem[] {
    if (element) {
      return [];
    }
    return [
      new PlaceholderItem(
        'No bundles yet',
        'Run "ctx: Export Workspace Context" to create one.',
        vscode.TreeItemCollapsibleState.None,
      ),
    ];
  }
}

class PlaceholderItem extends vscode.TreeItem {
  constructor(label: string, description: string, collapsible: vscode.TreeItemCollapsibleState) {
    super(label, collapsible);
    this.description = description;
    this.tooltip = description;
  }
}
