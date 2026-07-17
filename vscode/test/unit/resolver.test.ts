import { describe, it, expect } from 'vitest';
import { resolveBinaryPath, currentTarget } from '../../src/binary/resolver';

describe('resolver', () => {
  it('exposes the current platform target', () => {
    const t = currentTarget();
    // The test env runs on linux/amd64 in CI and locally; both are
    // supported. Anything else here would be a setup bug.
    expect(['linux', 'darwin', 'windows']).toContain(t.goos);
    expect(['amd64', 'arm64']).toContain(t.goarch);
  });

  it('resolves a binary path under <extension>/bin/', () => {
    const p = resolveBinaryPath('/fake/extension');
    expect(p).toMatch(/bin[\\/+]ctx-[a-z]+-[a-z0-9]+(\.exe)?$/);
  });

  it('uses .exe on windows-style goos', () => {
    // We can't change process.platform in a unit test, but we can
    // assert the resolver's behavior is stable on the current one.
    const p = resolveBinaryPath('/fake');
    if (process.platform === 'win32') {
      expect(p).toMatch(/\.exe$/);
    } else {
      expect(p).not.toMatch(/\.exe$/);
    }
  });
});
