import * as vscode from 'vscode';
import { CtxClient } from '../binary/client';
import { getCachedBinaryPath } from '../binary/resolver';
import { ensureExecutable, binaryExists } from '../util/fs';
import { getOutputChannel } from '../util/logger';
import { checkVersion } from '../binary/version';

let _cachedClient: CtxClient | undefined;
let _cachedExtPath: string | undefined;

/**
 * Returns a singleton CtxClient for the given extension, lazily creating
 * one on first use. Ensures the binary is executable (POSIX) on first
 * call. Throws if the binary is missing or incompatible.
 */
export async function getClient(extensionPath: string): Promise<CtxClient> {
  if (_cachedClient && _cachedExtPath === extensionPath) {
    return _cachedClient;
  }

  const binaryPath = getCachedBinaryPath(extensionPath);

  if (!binaryExists(binaryPath)) {
    throw new BinaryMissingError(binaryPath);
  }

  if (!ensureExecutable(binaryPath)) {
    throw new BinaryNotExecutableError(binaryPath);
  }

  const client = new CtxClient(binaryPath, getOutputChannel());

  // Verify compatibility on first creation. We don't hard-fail here —
  // surface a warning and let the user proceed at their own risk.
  try {
    const info = await client.version();
    const check = checkVersion(info);
    if (!check.compatible) {
      const action = await vscode.window.showWarningMessage(
        `ctx: ${check.message}`,
        'Show Logs',
        'Dismiss',
      );
      if (action === 'Show Logs') {
        getOutputChannel().show();
      }
    }
  } catch (err) {
    // Version check failures are non-fatal but should be visible.
    getOutputChannel().appendLine(
      `[warn] version check failed: ${err instanceof Error ? err.message : String(err)}`,
    );
  }

  _cachedClient = client;
  _cachedExtPath = extensionPath;
  return client;
}

/** Clears the cached client. Used by tests. */
export function resetClient(): void {
  _cachedClient = undefined;
  _cachedExtPath = undefined;
}

/** True iff a binary exists for the current platform. */
export function hasBinary(extensionPath: string): boolean {
  return binaryExists(getCachedBinaryPath(extensionPath));
}

export class BinaryMissingError extends Error {
  constructor(public readonly binaryPath: string) {
    super(
      `No ctx binary found at ${binaryPath}. The extension may have been ` +
        `installed incorrectly, or your platform isn't supported. ` +
        `Supported: linux/darwin/windows on amd64/arm64.`,
    );
    this.name = 'BinaryMissingError';
  }
}

export class BinaryNotExecutableError extends Error {
  constructor(public readonly binaryPath: string) {
    super(`ctx binary at ${binaryPath} exists but could not be made executable.`);
    this.name = 'BinaryNotExecutableError';
  }
}
