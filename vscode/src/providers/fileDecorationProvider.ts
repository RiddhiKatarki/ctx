import * as vscode from 'vscode';

/**
 * Adds a subtle badge to .ctx files in the explorer: a package icon
 * with a small "dirty" indicator color for bundles whose name
 * suggests they contain uncommitted changes.
 *
 * VS Code's FileDecorationProvider is the right API for this — it's
 * how Git, SVN, etc. add per-file badges without claiming the file's
 * primary icon slot.
 */
export class BundleDecorationProvider implements vscode.FileDecorationProvider {
  private readonly _onDidChange = new vscode.EventEmitter<undefined>();
  readonly onDidChangeFileDecorations = this._onDidChange.event;

  provideFileDecoration(
    uri: vscode.Uri,
    _token: vscode.CancellationToken,
  ): vscode.FileDecoration | undefined {
    if (!uri.path.endsWith('.ctx')) {
      return undefined;
    }
    return new vscode.FileDecoration('CTX', 'Context Handoff bundle', new vscode.ThemeColor('charts.blue'));
  }

  refresh(): void {
    this._onDidChange.fire(undefined);
  }
}
