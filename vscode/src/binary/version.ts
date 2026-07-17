import type { VersionInfo } from '../types';
import { BUNDLED_BINARY_VERSION, MIN_BINARY_VERSION } from '../constants';

/**
 * Result of a version check. `compatible` is the green/red signal;
 * `message` carries a user-presentable explanation in either case.
 */
export interface VersionCheck {
  info: VersionInfo;
  compatible: boolean;
  bundled: string;
  minimum: string;
  message: string;
}

/**
 * Compares a binary's reported version against the extension's
 * minimum-supported version. The minimum is what we promise to support;
 * the bundled version is informational.
 */
export function checkVersion(info: VersionInfo): VersionCheck {
  const compatible = gte(info.version, MIN_BINARY_VERSION);
  let message: string;
  if (compatible) {
    message = `ctx ${info.version} (${info.binary_os}/${info.binary_arch}), bundle format v${info.bundle_format}`;
  } else {
    message = `ctx binary ${info.version} is older than required ${MIN_BINARY_VERSION}; please update.`;
  }
  return {
    info,
    compatible,
    bundled: BUNDLED_BINARY_VERSION,
    minimum: MIN_BINARY_VERSION,
    message,
  };
}

/**
 * Trivial x.y.z comparator. Pre-release suffixes are compared
 * lexically and treated as LOWER than the same version without one
 * (per semver). Used only on version strings we produce ourselves.
 */
export function gte(a: string, b: string): boolean {
  return compareSemver(a, b) >= 0;
}

function compareSemver(a: string, b: string): number {
  const [aMain, aPre] = splitPre(a);
  const [bMain, bPre] = splitPre(b);
  const mainCmp = compareMain(aMain, bMain);
  if (mainCmp !== 0) return mainCmp;
  // No prerelease > has prerelease (semver rule).
  if (aPre === undefined && bPre === undefined) return 0;
  if (aPre === undefined) return 1;
  if (bPre === undefined) return -1;
  return aPre < bPre ? -1 : aPre > bPre ? 1 : 0;
}

function splitPre(v: string): [number[], string | undefined] {
  const [main, pre] = v.split('-', 2);
  const parts = main.split('.').map((n) => Number.parseInt(n, 10));
  return [parts, pre];
}

function compareMain(a: number[], b: number[]): number {
  const len = Math.max(a.length, b.length);
  for (let i = 0; i < len; i++) {
    const av = a[i] ?? 0;
    const bv = b[i] ?? 0;
    if (av !== bv) return av - bv;
  }
  return 0;
}
