import * as vscode from 'vscode';
import { CtxError } from '../binary/client';
import { Settings } from '../config/settings';
import { getOutputChannel } from '../util/logger';

/**
 * Routes a CtxError to the right notification severity based on its
 * code, honoring the user's ctx.showNotifications preference.
 *
 *   user_error      → showWarningMessage
 *   system_error    → showErrorMessage + "Open Logs" action
 *   invalid_bundle  → showErrorMessage
 */
export function reportError(err: unknown, settings: Settings): void {
  if (!(err instanceof CtxError)) {
    if (settings.showNotifications !== 'none') {
      void vscode.window.showErrorMessage(surpriseError(err));
    }
    return;
  }

  if (settings.showNotifications === 'none') {
    return;
  }

  switch (err.code) {
    case 'user_error':
      if (settings.showNotifications === 'errorsOnly') {
        // Errors-only suppresses warnings; but user errors are still
        // surfaced because they indicate the command didn't run.
        void vscode.window.showWarningMessage(`ctx: ${err.message}`);
      } else {
        void vscode.window.showWarningMessage(`ctx: ${err.message}`);
      }
      break;
    case 'invalid_bundle':
      void vscode.window.showErrorMessage(`ctx: ${err.message}`);
      break;
    case 'system_error':
    default:
      void vscode.window
        .showErrorMessage(`ctx: ${err.message}`, 'Open Logs')
        .then((action) => {
          if (action === 'Open Logs') {
            getOutputChannel().show();
          }
        });
      break;
  }
}

/**
 * Reports a successful operation. Suppressed unless the user opted
 * into 'all' notifications. Use this for background or implicit
 * successes (e.g. auto-refresh) where a toast would be noisy.
 */
export function reportSuccess(message: string, settings: Settings): void {
  if (settings.showNotifications === 'all') {
    void vscode.window.showInformationMessage(`ctx: ${message}`);
  }
}

/**
 * Reports a successful user-initiated action. ALWAYS shows a
 * notification (regardless of ctx.showNotifications) because the
 * user explicitly invoked the command and expects feedback. Optional
 * action buttons can be provided; clicking one resolves the returned
 * promise with the button label.
 *
 * Use this for the final step of commands like import/export where
 * silence would be confusing.
 */
export async function reportUserActionSuccess(
  message: string,
  ...buttons: string[]
): Promise<string | undefined> {
  return vscode.window.showInformationMessage(`ctx: ${message}`, ...buttons);
}

function surpriseError(err: unknown): string {
  if (err instanceof Error) {
    return `ctx: unexpected error: ${err.message}`;
  }
  return `ctx: unexpected error: ${String(err)}`;
}
