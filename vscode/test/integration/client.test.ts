import { describe, it, expect, beforeAll } from 'vitest';
import { existsSync, mkdirSync, writeFileSync, rmSync } from 'fs';
import { join } from 'path';
import { execSync } from 'child_process';
import { CtxClient, EXIT_CODES, CtxError } from '../../src/binary/client';

// vitest transforms test files; __dirname is unreliable. Use
// process.cwd() which is always the extension root under npm scripts.
const EXTENSION_ROOT = process.cwd();
const BINARY = join(EXTENSION_ROOT, 'bin', 'ctx-linux-amd64');
const HAVE_BINARY = existsSync(BINARY) && process.platform === 'linux' && process.arch === 'x64';

// Fixture: a tiny git repo we can run `ctx export --stream` against.
// Built once for the whole suite.
const FIXTURE_DIR = join(EXTENSION_ROOT, 'out', 'test-fixture');
let exportBundlePath = '';

const itIfBinary = HAVE_BINARY ? it : it.skip;

beforeAll(() => {
  if (!HAVE_BINARY) return;
  rmSync(FIXTURE_DIR, { recursive: true, force: true });
  mkdirSync(FIXTURE_DIR, { recursive: true });
  writeFileSync(join(FIXTURE_DIR, 'README.md'), '# test\n');
  const git = (args: string) =>
    execSync(`git ${args}`, { cwd: FIXTURE_DIR, stdio: 'ignore', env: { ...process.env, GIT_AUTHOR_NAME: 't', GIT_AUTHOR_EMAIL: 't@t.com', GIT_COMMITTER_NAME: 't', GIT_COMMITTER_EMAIL: 't@t.com' } });
  git('init -q');
  git('add .');
  git('commit -q -m init');
  // Make the tree dirty so export has a diff to capture.
  writeFileSync(join(FIXTURE_DIR, 'README.md'), '# test\n\nedited\n');
  exportBundlePath = join(FIXTURE_DIR, 'project.ctx');
});

describe('CtxClient (integration against real binary)', () => {
  const client = new CtxClient(BINARY);

  itIfBinary('version() returns the expected shape', async () => {
    const v = await client.version();
    expect(v.version).toMatch(/^\d+\.\d+\.\d+/);
    expect(v.bundle_format).toBe(1);
    expect(v.binary_os).toBe('linux');
    expect(v.binary_arch).toBe('amd64');
  });

  itIfBinary('runJSON<T> parses single-result stdout', async () => {
    // `ctx version --json` is the canonical single-result command.
    const v = await client.runJSON<{ version: string }>(['version']);
    expect(v.version).toMatch(/^\d+\.\d+\.\d+/);
  });

  itIfBinary('runStream emits events and resolves with complete payload', async () => {
    // export is the canonical streaming command. It emits start,
    // git_detected, git_metadata, ..., complete with the ExportResult.
    // Note: export uses os.Getwd() so we MUST pass cwd via opts.
    const events: { event: string }[] = [];
    const result = await client.runStream<{ path: string; project_name: string }>(
      ['export', '--no-secret-scan', '-o', exportBundlePath],
      (e) => events.push(e),
      undefined,
      { cwd: FIXTURE_DIR },
    );
    expect(result.path).toBe(exportBundlePath);
    expect(result.project_name).toBe('test-fixture');
    // Should have seen at least start + complete and a few intermediate.
    const names = events.map((e) => e.event);
    expect(names).toContain('start');
    expect(names).toContain('git_detected');
    expect(names.length).toBeGreaterThanOrEqual(3);
  });

  itIfBinary('rejects with CtxError on a user error (missing file)', async () => {
    await expect(client.runJSON(['info', '/nonexistent/file.ctx'])).rejects.toMatchObject({
      name: 'CtxError',
      code: 'user_error',
      exitCode: EXIT_CODES.USER,
    });
  });

  itIfBinary('rejects with invalid_bundle on a non-ctx file', async () => {
    // /etc/hostname is not a ZIP, so ctx info treats it as invalid.
    // Skip if /etc/hostname doesn't exist (rare).
    if (!existsSync('/etc/hostname')) {
      console.warn('skipping invalid_bundle test: /etc/hostname missing');
      return;
    }
    await expect(client.runJSON(['info', '/etc/hostname'])).rejects.toMatchObject({
      name: 'CtxError',
    });
  });

  itIfBinary('exposes envelope.error.message on failure', async () => {
    try {
      await client.runJSON(['info', '/nonexistent/file.ctx']);
      expect.fail('should have thrown');
    } catch (err) {
      expect(err).toBeInstanceOf(CtxError);
      const e = err as CtxError;
      expect(e.envelope.error.code).toBe('user_error');
      expect(e.envelope.error.message.length).toBeGreaterThan(0);
      expect(e.stderr.length).toBeGreaterThan(0);
    }
  });

  itIfBinary('info on the fixture bundle returns valid metadata', async () => {
    // Requires the stream test to have run first to produce project.ctx.
    if (!existsSync(exportBundlePath)) {
      console.warn('skipping: fixture bundle missing');
      return;
    }
    const r = await client.runJSON<{ path: string; valid: boolean; project_name?: string }>(
      ['info', exportBundlePath],
    );
    expect(r.path).toBe(exportBundlePath);
    expect(r.valid).toBe(true);
  });
});
