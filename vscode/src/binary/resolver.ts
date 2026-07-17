import * as path from 'path';

/**
 * Map (process.platform, process.arch) to the bundled ctx binary.
 *
 * Binaries live in <extension>/bin/ and are named per the CI matrix:
 *   ctx-<goos>-<goarch>[.exe]
 *
 * Throws a helpful error if no binary exists for the current platform.
 */
export function resolveBinaryPath(extensionPath: string): string {
  const { goos, goarch } = currentTarget();
  const exe = goos === 'windows' ? '.exe' : '';
  const name = `ctx-${goos}-${goarch}${exe}`;
  return path.join(extensionPath, 'bin', name);
}

/**
 * Returns the Go-style (GOOS, GOARCH) tuple for the running process.
 * Throws if the host isn't one of the 6 supported combinations.
 */
export function currentTarget(): { goos: string; goarch: string } {
  const goos = platformToGoos(process.platform);
  const goarch = archToGoarch(process.arch);
  if (!goos) {
    throw new Error(
      `Unsupported platform: ${process.platform}. ctx supports linux, darwin, and windows.`,
    );
  }
  if (!goarch) {
    throw new Error(
      `Unsupported architecture: ${process.arch}. ctx supports amd64 and arm64.`,
    );
  }
  return { goos, goarch };
}

function platformToGoos(p: NodeJS.Platform): string | undefined {
  switch (p) {
    case 'linux':
      return 'linux';
    case 'darwin':
      return 'darwin';
    case 'win32':
      return 'windows';
    default:
      return undefined;
  }
}

function archToGoarch(a: string): string | undefined {
  switch (a) {
    case 'x64':
      return 'amd64';
    case 'arm64':
      return 'arm64';
    default:
      return undefined;
  }
}

/**
 * Cached {@link resolveBinaryPath} result keyed by extensionPath.
 * Reset by VS Code automatically on extension reload.
 */
let _cached: { ext: string; path: string } | undefined;

export function getCachedBinaryPath(extensionPath: string): string {
  if (_cached && _cached.ext === extensionPath) {
    return _cached.path;
  }
  const p = resolveBinaryPath(extensionPath);
  _cached = { ext: extensionPath, path: p };
  return p;
}
