import * as vscode from 'vscode';
import { OUTPUT_CHANNEL_NAME } from '../constants';

let _channel: vscode.OutputChannel | undefined;

export function getOutputChannel(): vscode.OutputChannel {
  if (!_channel) {
    _channel = vscode.window.createOutputChannel(OUTPUT_CHANNEL_NAME);
  }
  return _channel;
}

export function log(msg: string): void {
  getOutputChannel().appendLine(msg);
}

export function logError(prefix: string, err: unknown): void {
  const msg = err instanceof Error ? `${err.message}${err.stack ? `\n${err.stack}` : ''}` : String(err);
  getOutputChannel().appendLine(`[error] ${prefix}: ${msg}`);
}
