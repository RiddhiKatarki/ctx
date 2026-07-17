import { describe, it, expect } from 'vitest';
import { existsSync, mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'fs';
import { tmpdir } from 'os';
import { join } from 'path';

// Mirror the helpers in src/commands/export.ts. They're not exported,
// so we replicate them here. The integration test in pipeline.test.ts
// exercises the real ones end-to-end against the binary; this file
// covers the pure FS heuristics (which are easier to test in isolation).

function isGitRepo(dir: string): boolean {
  let current = require('path').resolve(dir);
  for (;;) {
    if (existsSync(join(current, '.git'))) {
      return true;
    }
    const parent = require('path').dirname(current);
    if (parent === current) return false;
    current = parent;
  }
}

function findGitSubfolder(root: string): string | undefined {
  const fs = require('fs');
  const path = require('path');
  let entries: string[];
  try {
    entries = fs.readdirSync(root);
  } catch {
    return undefined;
  }
  const candidates: string[] = [];
  for (const entry of entries) {
    const full = path.join(root, entry);
    let stat;
    try {
      stat = fs.statSync(full);
    } catch {
      continue;
    }
    if (stat.isDirectory() && !entry.startsWith('.') && entry !== 'node_modules') {
      if (existsSync(join(full, '.git'))) {
        candidates.push(full);
      }
    }
  }
  return candidates.length === 1 ? candidates[0] : undefined;
}

describe('export pre-flight heuristics', () => {
  let tmpRoot: string;

  it('isGitRepo walks up parent directories', () => {
    tmpRoot = mkdtempSync(join(tmpdir(), 'ctx-git-'));
    try {
      const child = join(tmpRoot, 'a', 'b', 'c');
      mkdirSync(child, { recursive: true });
      // No .git anywhere → false
      expect(isGitRepo(child)).toBe(false);

      // Create .git at tmpRoot → child should detect it
      mkdirSync(join(tmpRoot, '.git'));
      expect(isGitRepo(child)).toBe(true);
      expect(isGitRepo(tmpRoot)).toBe(true);
    } finally {
      rmSync(tmpRoot, { recursive: true, force: true });
    }
  });

  it('findGitSubfolder returns the only candidate', () => {
    tmpRoot = mkdtempSync(join(tmpdir(), 'ctx-sub-'));
    try {
      mkdirSync(join(tmpRoot, 'repo-a', '.git'), { recursive: true });
      mkdirSync(join(tmpRoot, 'not-a-repo'));
      writeFileSync(join(tmpRoot, 'not-a-repo', 'file.txt'), 'x');
      const found = findGitSubfolder(tmpRoot);
      expect(found).toBe(join(tmpRoot, 'repo-a'));
    } finally {
      rmSync(tmpRoot, { recursive: true, force: true });
    }
  });

  it('findGitSubfolder returns undefined when multiple or zero candidates exist', () => {
    tmpRoot = mkdtempSync(join(tmpdir(), 'ctx-multi-'));
    try {
      // Zero candidates
      expect(findGitSubfolder(tmpRoot)).toBeUndefined();

      // Two candidates → ambiguous, return undefined
      mkdirSync(join(tmpRoot, 'repo-a', '.git'), { recursive: true });
      mkdirSync(join(tmpRoot, 'repo-b', '.git'), { recursive: true });
      expect(findGitSubfolder(tmpRoot)).toBeUndefined();
    } finally {
      rmSync(tmpRoot, { recursive: true, force: true });
    }
  });

  it('findGitSubfolder ignores node_modules and dotfiles', () => {
    tmpRoot = mkdtempSync(join(tmpdir(), 'ctx-ignore-'));
    try {
      // A node_modules with .git inside (common when packages are
      // cloned for development) should not be picked.
      mkdirSync(join(tmpRoot, 'node_modules', 'some-pkg', '.git'), { recursive: true });
      mkdirSync(join(tmpRoot, '.cache', '.git'), { recursive: true });
      expect(findGitSubfolder(tmpRoot)).toBeUndefined();
    } finally {
      rmSync(tmpRoot, { recursive: true, force: true });
    }
  });
});
