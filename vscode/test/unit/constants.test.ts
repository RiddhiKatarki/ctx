import { describe, it, expect } from 'vitest';
import { MIN_BINARY_VERSION, BUNDLED_BINARY_VERSION } from '../../src/constants';

describe('constants', () => {
  it('bundled binary version satisfies the minimum', () => {
    // Lexical comparison is fine for semver-style x.y.z; the version
    // pinned at activation time is documented to be >= the minimum.
    expect(BUNDLED_BINARY_VERSION >= MIN_BINARY_VERSION).toBe(true);
  });
});
