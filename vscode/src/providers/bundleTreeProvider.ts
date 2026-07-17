import * as vscode from 'vscode';
import type { CtxClient } from '../binary/client';
import type { BundleEntry, ListResult } from '../types';

/**
 * TreeDataProvider that lists .ctx bundles in the first workspace folder.
 * Refresh by calling .refresh() (wired to ctx.list.refresh command).
 */
export class BundleTreeProvider implements vscode.TreeDataProvider<BundleEntry> {
  private readonly _onDidChange = new vscode.EventEmitter<BundleEntry | undefined>();
  readonly onDidChangeTreeData = this._onDidChange.event;

  constructor(
    private readonly getClient: () => Promise<CtxClient>,
    private readonly onChangeWatcher?: vscode.Disposable,
  ) {}

  refresh(): void {
    this._onDidChange.fire(undefined);
  }

  getTreeItem(entry: BundleEntry): vscode.TreeItem {
    const item = new vscode.TreeItem(entry.name);
    item.description = describeEntry(entry);
    item.tooltip = tooltipFor(entry);
    item.iconPath = new vscode.ThemeIcon('package');
    item.contextValue = 'ctx.bundle';
    item.command = {
      command: 'ctx.inspect',
      title: 'Inspect',
      arguments: [vscode.Uri.file(entry.path)],
    };
    return item;
  }

  async getChildren(element?: BundleEntry): Promise<BundleEntry[]> {
    if (element) {
      return []; // leaves
    }
    const dir = firstWorkspaceFolder();
    if (!dir) {
      return [];
    }
    try {
      const client = await this.getClient();
      const result = await client.runJSON<ListResult>(['list', dir]);
      // Sort newest first (discovery already does this, but be defensive).
      return result.bundles.slice().sort((a, b) => b.created_at.localeCompare(a.created_at));
    } catch {
      // Silent failure — the tree just shows empty. Errors propagate
      // through notifications only when the user explicitly runs a command.
      return [];
    }
  }

  dispose(): void {
    this._onDidChange.dispose();
    this.onChangeWatcher?.dispose();
  }
}

function firstWorkspaceFolder(): string | undefined {
  const folders = vscode.workspace.workspaceFolders;
  return folders?.[0]?.uri.fsPath;
}

function describeEntry(e: BundleEntry): string {
  const parts: string[] = [];
  parts.push(e.branch || '(no branch)');
  parts.push(`${e.file_count} file${e.file_count === 1 ? '' : 's'}`);
  if (e.dirty) {
    parts.push('dirty');
  }
  return parts.join(', ');
}

function tooltipFor(e: BundleEntry): string {
  const lines = [
    e.project_name || e.name,
    `Branch: ${e.branch || '(none)'}`,
    `Files: ${e.file_count}`,
    `Dirty: ${e.dirty ? 'yes' : 'no'}`,
    e.head_commit ? `HEAD: ${e.head_commit.slice(0, 8)}` : '',
    `Created: ${e.created_at}`,
  ].filter(Boolean);
  return lines.join('\n');
}
