import * as vscode from 'vscode';
import { SecretStore } from '../config/settings';
import { log } from '../util/logger';

/**
 * ctx.updateApiKey command. Lets the user view key status, replace, or
 * delete the stored OpenAI API key without going through the export flow.
 *
 * The stored key value is never shown (it's a secret). Instead we report
 * only whether one is set, then offer to replace or clear it.
 */
export function makeUpdateApiKeyCommand(secrets: SecretStore) {
  return async (): Promise<void> => {
    const has = await secrets.getApiKey();

    if (has) {
      const action = await vscode.window.showInformationMessage(
        'An OpenAI API key is currently stored in SecretStorage.',
        'Replace',
        'Delete',
        'Cancel',
      );
      if (action === 'Replace') {
        await promptAndStore(secrets);
      } else if (action === 'Delete') {
        await secrets.deleteApiKey();
        log('api key deleted from SecretStorage');
        void vscode.window.showInformationMessage('ctx: API key deleted.');
      }
      return;
    }

    // No key set yet — go straight to the prompt.
    await promptAndStore(secrets);
  };
}

async function promptAndStore(secrets: SecretStore): Promise<void> {
  const key = await vscode.window.showInputBox({
    prompt: 'OpenAI API key',
    placeHolder: 'sk-...',
    password: true,
    ignoreFocusOut: true,
    validateInput: (v) => (v.trim().length === 0 ? 'Key cannot be empty' : undefined),
  });
  if (key === undefined) {
    return; // user cancelled
  }
  await secrets.setApiKey(key.trim());
  log('api key updated in SecretStorage');
  void vscode.window.showInformationMessage('ctx: API key saved.');
}
