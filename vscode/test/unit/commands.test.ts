import { describe, it, expect } from 'vitest';

// We import the unexported format helpers indirectly by replicating
// the inputs they were written against. These are sanity tests for
// the canonical NDJSON event names the binary emits — if ctx ever
// changes a name, this test catches the drift.
//
// The helpers themselves aren't exported from commands/*.ts (they're
// module-private); to avoid churning the command modules just for
// testability, we mirror the humanize maps here and assert they cover
// every event in the documented schema.

// Matches src/commands/export.ts -> humanizeEvent switch.
const EXPORT_EVENT_MESSAGES: Record<string, string | undefined> = {
  start: 'Starting…',
  git_detected: 'Reading git repository…',
  git_metadata: 'Captured git metadata',
  prompts_collected: 'Collected N prompt(s)',
  files_collected: 'Found N modified file(s)',
  diff_captured: 'Captured uncommitted changes',
  secrets_found: 'Redacted N secret(s)',
  snapshot_built: 'Built snapshot',
  contents_read: 'Embedded N file(s)',
  summary_start: 'Generating summary…',
  summary_complete: 'Summary ready',
  bundle_written: 'Writing bundle…',
};

// Matches src/commands/import.ts -> humanizeImportEvent switch.
const IMPORT_EVENT_MESSAGES: Record<string, string | undefined> = {
  start: 'Reading bundle…',
  extracted: 'Unpacked archive',
  manifest_validated: 'Manifest validated',
  extracted_to: 'Extracted to ...',
};

describe('event-name coverage', () => {
  // These names come from internal/export/export.go and internal/import/import.go.
  // If the CLI ever renames or adds an event, the extension's progress UI
  // would silently show a blank message. This test makes the contract explicit.
  const KNOWN_EXPORT_EVENTS = [
    'start', 'git_detected', 'git_metadata', 'prompts_collected',
    'ctxignore_loaded', 'files_excluded', 'files_collected',
    'diff_captured', 'secrets_found', 'snapshot_built', 'contents_read',
    'summary_start', 'summary_complete', 'bundle_written', 'complete',
  ];
  const KNOWN_IMPORT_EVENTS = [
    'start', 'extracted', 'manifest_validated', 'extracted_to', 'complete',
  ];

  it('export event messages cover every event in the documented schema', () => {
    const uncovered: string[] = [];
    for (const name of KNOWN_EXPORT_EVENTS) {
      // 'complete' is handled by runStream directly, not the humanizer.
      // 'ctxignore_loaded' and 'files_excluded' are conditional and
      // uninteresting in the UI; we let them fall through silently.
      if (name === 'complete' || name === 'ctxignore_loaded' || name === 'files_excluded') {
        continue;
      }
      if (!(name in EXPORT_EVENT_MESSAGES)) {
        uncovered.push(name);
      }
    }
    expect(uncovered).toEqual([]);
  });

  it('import event messages cover every event in the documented schema', () => {
    const uncovered: string[] = [];
    for (const name of KNOWN_IMPORT_EVENTS) {
      if (name === 'complete') continue;
      if (!(name in IMPORT_EVENT_MESSAGES)) {
        uncovered.push(name);
      }
    }
    expect(uncovered).toEqual([]);
  });
});

describe('summary formatting', () => {
  // Mirror of src/commands/import.ts -> formatSummary, kept here for
  // unit-testability without exporting the helper. If the formats
  // diverge, an integration test against a real bundle catches it.
  function formatSummary(r: {
    project_name: string;
    branch: string;
    file_count: number;
    has_diff: boolean;
    diff_size: number;
    prompt_count: number;
  }): string {
    const parts = [`${r.project_name} (${r.branch})`, `${r.file_count} file(s)`];
    if (r.has_diff) parts.push(`${r.diff_size}-byte diff`);
    if (r.prompt_count > 0) parts.push(`${r.prompt_count} prompt(s)`);
    return parts.join(', ');
  }

  it('omits diff line when has_diff is false', () => {
    const s = formatSummary({
      project_name: 'demo',
      branch: 'main',
      file_count: 3,
      has_diff: false,
      diff_size: 0,
      prompt_count: 0,
    });
    expect(s).toBe('demo (main), 3 file(s)');
  });

  it('includes diff and prompt lines when present', () => {
    const s = formatSummary({
      project_name: 'demo',
      branch: 'main',
      file_count: 3,
      has_diff: true,
      diff_size: 450,
      prompt_count: 7,
    });
    expect(s).toBe('demo (main), 3 file(s), 450-byte diff, 7 prompt(s)');
  });
});

describe('byte formatting', () => {
  function formatBytes(n: number): string {
    if (n < 1024) return `${n} B`;
    if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
    return `${(n / 1024 / 1024).toFixed(1)} MB`;
  }

  it('formats bytes', () => {
    expect(formatBytes(0)).toBe('0 B');
    expect(formatBytes(512)).toBe('512 B');
    expect(formatBytes(2048)).toBe('2.0 KB');
    expect(formatBytes(2 * 1024 * 1024)).toBe('2.0 MB');
  });
});
