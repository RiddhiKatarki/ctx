import { describe, it, expect } from 'vitest';
import { checkVersion, gte } from '../../src/binary/version';
import type { VersionInfo } from '../../src/types';

function mk(version: string): VersionInfo {
  return { version, bundle_format: 1, binary_os: 'linux', binary_arch: 'amd64' };
}

describe('version', () => {
  describe('gte', () => {
    it('compares major', () => {
      expect(gte('2.1.0', '1.99.99')).toBe(true);
      expect(gte('1.99.99', '2.0.0')).toBe(false);
    });

    it('compares minor when major equal', () => {
      expect(gte('2.1.0', '2.0.5')).toBe(true);
      expect(gte('2.0.5', '2.1.0')).toBe(false);
    });

    it('compares patch when major and minor equal', () => {
      expect(gte('2.1.5', '2.1.0')).toBe(true);
      expect(gte('2.1.0', '2.1.5')).toBe(false);
    });

    it('equal versions are gte', () => {
      expect(gte('2.1.0', '2.1.0')).toBe(true);
    });

    it('prerelease is lower than release', () => {
      expect(gte('2.1.0', '2.1.0-rc.1')).toBe(true);
      expect(gte('2.1.0-rc.1', '2.1.0')).toBe(false);
    });
  });

  describe('checkVersion', () => {
    it('marks a fresh binary as compatible', () => {
      const r = checkVersion(mk('2.1.0'));
      expect(r.compatible).toBe(true);
      expect(r.message).toContain('2.1.0');
    });

    it('marks an older binary as incompatible', () => {
      const r = checkVersion(mk('2.0.0'));
      expect(r.compatible).toBe(false);
      expect(r.message).toContain('older than required');
    });
  });
});
