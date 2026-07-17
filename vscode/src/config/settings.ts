import * as vscode from 'vscode';
import { CONFIG, SECRET_KEYS } from '../constants';

/**
 * Reads the ctx.* configuration namespace. Wraps getConfiguration so
 * command code doesn't repeat the section name.
 */
export class Settings {
  constructor(private readonly config: vscode.WorkspaceConfiguration) {}

  static create(): Settings {
    return new Settings(vscode.workspace.getConfiguration('ctx'));
  }

  get summaryProvider(): 'template' | 'openai' {
    return this.config.get<'template' | 'openai'>(stripPrefix(CONFIG.summaryProvider)) as 'template' | 'openai';
  }
  get openaiBaseUrl(): string {
    return this.config.get<string>(stripPrefix(CONFIG.openaiBaseUrl)) as string;
  }
  get openaiModel(): string {
    return this.config.get<string>(stripPrefix(CONFIG.openaiModel)) as string;
  }
  get defaultOutputName(): string {
    return this.config.get<string>(stripPrefix(CONFIG.defaultOutputName)) as string;
  }
  get defaultOutdir(): string {
    return this.config.get<string>(stripPrefix(CONFIG.defaultOutdir)) as string;
  }
  get secretScanEnabled(): boolean {
    return this.config.get<boolean>(stripPrefix(CONFIG.secretScanEnabled)) as boolean;
  }
  get includeContents(): boolean {
    return this.config.get<boolean>(stripPrefix(CONFIG.includeContents)) as boolean;
  }
  get contentsThreshold(): number {
    return this.config.get<number>(stripPrefix(CONFIG.contentsThreshold)) as number;
  }
  get showNotifications(): 'all' | 'errorsOnly' | 'none' {
    return this.config.get<'all' | 'errorsOnly' | 'none'>(stripPrefix(CONFIG.showNotifications)) as 'all' | 'errorsOnly' | 'none';
  }
}

function stripPrefix(key: string): string {
  return key.startsWith('ctx.') ? key.slice(4) : key;
}

/**
 * Wrapper around VS Code's SecretStorage for the OpenAI API key.
 * The key never touches settings.json or git.
 */
export class SecretStore {
  constructor(private readonly secrets: vscode.SecretStorage) {}

  async getApiKey(): Promise<string | undefined> {
    return this.secrets.get(SECRET_KEYS.openaiApiKey);
  }

  async setApiKey(key: string): Promise<void> {
    await this.secrets.store(SECRET_KEYS.openaiApiKey, key);
  }

  async deleteApiKey(): Promise<void> {
    await this.secrets.delete(SECRET_KEYS.openaiApiKey);
  }

  /**
   * Prompts the user for an API key if none is stored. Returns the key
   * or undefined if the user cancelled. Stores on first successful entry.
   */
  async ensureApiKey(): Promise<string | undefined> {
    let key = await this.getApiKey();
    if (key) {
      return key;
    }
    key = await vscode.window.showInputBox({
      prompt: 'OpenAI API key',
      placeHolder: 'sk-...',
      password: true,
      ignoreFocusOut: true,
      validateInput: (v) => (v.trim().length === 0 ? 'Key cannot be empty' : undefined),
    });
    if (key) {
      await this.setApiKey(key.trim());
    }
    return key;
  }
}
