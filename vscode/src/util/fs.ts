import * as fs from 'fs';

/**
 * Ensures a bundled binary is executable on POSIX systems. On Windows
 * the exec bit is irrelevant and we no-op.
 *
 * Safe to call repeatedly; idempotent. Resolves true if the file is
 * already executable or the chmod succeeded.
 *
 * VS Code's .vsix extraction does not always preserve file modes, so
 * we run this once per activation on the resolved binary.
 */
export function ensureExecutable(binaryPath: string): boolean {
  if (process.platform === 'win32') {
    return true;
  }
  try {
    const stat = fs.statSync(binaryPath);
    const ownerExec = (stat.mode & 0o100) !== 0;
    if (ownerExec) {
      return true;
    }
    fs.chmodSync(binaryPath, stat.mode | 0o100);
    return true;
  } catch {
    return false;
  }
}

/**
 * Returns true iff the resolved binary path actually points to a file
 * on disk. Useful for surfacing "binary not bundled" errors.
 */
export function binaryExists(binaryPath: string): boolean {
  try {
    return fs.statSync(binaryPath).isFile();
  } catch {
    return false;
  }
}
