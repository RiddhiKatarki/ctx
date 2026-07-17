import { describe, it, expect, beforeAll, afterAll } from 'vitest';
import { existsSync, mkdirSync, writeFileSync, rmSync, readdirSync } from 'fs';
import { join } from 'path';
import { execSync } from 'child_process';
import { CtxClient } from '../../src/binary/client';
import type {
  ExportResult,
  InspectResult,
  ImportResult,
  InfoResult,
} from '../../src/types';

const EXTENSION_ROOT = process.cwd();
const BINARY = join(EXTENSION_ROOT, 'bin', 'ctx-linux-amd64');
const HAVE_BINARY = existsSync(BINARY) && process.platform === 'linux' && process.arch === 'x64';

const itIfBinary = HAVE_BINARY ? it : it.skip;

// Fixture: a realistic git repo with multiple commits and dirty changes,
// so every event in the export pipeline has real content.
const FIXTURE_DIR = join(EXTENSION_ROOT, 'out', 'pipeline-fixture');
const BUNDLE_PATH = join(FIXTURE_DIR, 'project.ctx');
const IMPORT_OUT = join(FIXTURE_DIR, 'extracted');

let exportResult: ExportResult | undefined;

beforeAll(() => {
  if (!HAVE_BINARY) return;
  rmSync(FIXTURE_DIR, { recursive: true, force: true });
  mkdirSync(FIXTURE_DIR, { recursive: true });

  writeFileSync(join(FIXTURE_DIR, 'main.go'), 'package main\n\nfunc main() {}\n');
  writeFileSync(join(FIXTURE_DIR, 'README.md'), '# fixture\n');

  const git = (args: string) =>
    execSync(`git ${args}`, {
      cwd: FIXTURE_DIR,
      stdio: 'ignore',
      env: {
        ...process.env,
        GIT_AUTHOR_NAME: 't', GIT_AUTHOR_EMAIL: 't@t.com',
        GIT_COMMITTER_NAME: 't', GIT_COMMITTER_EMAIL: 't@t.com',
      },
    });
  git('init -q');
  git('add .');
  git('commit -q -m "initial"');
  git('checkout -q -b feature/test');

  // Make dirty changes so export has a diff.
  writeFileSync(join(FIXTURE_DIR, 'main.go'), 'package main\n\nfunc main() { println("hi") }\n');
  writeFileSync(join(FIXTURE_DIR, 'NEW.md'), '# new file\n');
});

afterAll(() => {
  // Keep the fixture for inspection on failure; clean on success.
  // rmSync(FIXTURE_DIR, { recursive: true, force: true });
});

describe('end-to-end pipeline: export -> info -> inspect -> import', () => {
  const client = new CtxClient(BINARY);

  itIfBinary('export produces a bundle', async () => {
    exportResult = await client.runStream<ExportResult>(
      ['export', '--no-secret-scan', '-o', BUNDLE_PATH],
      () => undefined,
      undefined,
      { cwd: FIXTURE_DIR },
    );
    expect(exportResult.path).toBe(BUNDLE_PATH);
    expect(exportResult.file_count).toBeGreaterThan(0);
    expect(existsSync(BUNDLE_PATH)).toBe(true);
  });

  itIfBinary('info on the produced bundle reports valid metadata', async () => {
    if (!exportResult) expect.fail('exportResult unset — earlier test failed');
    const info = await client.runJSON<InfoResult>(['info', BUNDLE_PATH]);
    expect(info.path).toBe(BUNDLE_PATH);
    expect(info.valid).toBe(true);
    expect(info.file_count).toBe(exportResult.file_count);
    expect(info.metadata).toMatchObject({
      project_name: 'pipeline-fixture',
      branch: 'feature/test',
    });
  });

  itIfBinary('inspect returns the 9 summary sections', async () => {
    const inspect = await client.runJSON<InspectResult>(['inspect', BUNDLE_PATH]);
    expect(inspect.valid).toBe(true);
    // The 9 canonical sections from internal/schema/schema.go.
    expect(Object.keys(inspect.summary_sections).sort()).toEqual([
      'architecture_decisions',
      'completed_work',
      'current_objective',
      'estimated_reading_time',
      'files_to_read_first',
      'known_bugs',
      'previous_failed_approaches',
      'remaining_tasks',
      'suggested_next_prompt',
    ]);
  });

  itIfBinary('import --outdir extracts the bundle', async () => {
    const result = await client.runJSON<ImportResult>([
      'import', BUNDLE_PATH, '--outdir', IMPORT_OUT,
    ]);
    expect(result.valid).toBe(true);
    expect(result.extracted_to).toBe(IMPORT_OUT);
    // All 7 canonical files should be present.
    const entries = readdirSync(IMPORT_OUT);
    expect(entries).toContain('manifest.json');
    expect(entries).toContain('metadata.json');
    expect(entries).toContain('git.json');
    expect(entries).toContain('summary.md');
    expect(entries).toContain('prompts.json');
    expect(entries).toContain('files.json');
    expect(entries).toContain('patch.diff');
  });

  itIfBinary('import on a non-bundle file fails with invalid_bundle', async () => {
    const junk = join(FIXTURE_DIR, 'not-a-bundle.ctx');
    writeFileSync(junk, 'this is not a zip file');
    await expect(client.runJSON(['import', junk])).rejects.toMatchObject({
      name: 'CtxError',
    });
  });
});
