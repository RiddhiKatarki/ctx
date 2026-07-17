#!/usr/bin/env node
/**
 * Downloads prebuilt ctx binaries from GitHub Releases into bin/.
 *
 * Modes:
 *   node scripts/download-bins.js             # all 6 targets (for packaging)
 *   node scripts/download-bins.js --dev       # only the current platform (for dev)
 *
 * The release tag is determined by BUNDLED_BINARY_VERSION in src/constants.ts.
 * Override with CTX_BINARY_VERSION env var.
 *
 * Re-run this after `npm install` and before pressing F5.
 */

const fs = require('fs');
const path = require('path');
const os = require('os');
const https = require('https');
const { execSync } = require('child_process');

const REPO = 'RiddhiKatarki/ctx';
const EXTENSION_ROOT = path.resolve(__dirname, '..');
const BIN_DIR = path.join(EXTENSION_ROOT, 'bin');

// Read the bundled binary version from constants.ts without compiling it.
function readBundledVersion() {
  if (process.env.CTX_BINARY_VERSION) {
    return process.env.CTX_BINARY_VERSION;
  }
  const constants = fs.readFileSync(
    path.join(EXTENSION_ROOT, 'src', 'constants.ts'),
    'utf8',
  );
  const m = constants.match(/BUNDLED_BINARY_VERSION\s*=\s*['"]([^'"]+)['"]/);
  if (!m) {
    throw new Error('could not find BUNDLED_BINARY_VERSION in src/constants.ts');
  }
  return m[1];
}

// Translate (goos, goarch) -> asset filename used by release.yml.
function assetName(goos, goarch) {
  const exe = goos === 'windows' ? '.exe' : '';
  return `ctx-${goos}-${goarch}${exe}`;
}

function currentPlatform() {
  const goos = process.platform === 'darwin' ? 'darwin' : process.platform;
  const goarch = process.arch;
  return { goos, goarch };
}

function fetch(url) {
  return new Promise((resolve, reject) => {
    https.get(url, { headers: { 'User-Agent': 'ctx-vscode-ext-download-bins' } }, (res) => {
      if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
        return resolve(fetch(res.headers.location));
      }
      if (res.statusCode !== 200) {
        return reject(new Error(`HTTP ${res.statusCode} for ${url}`));
      }
      resolve(res);
    }).on('error', reject);
  });
}

async function downloadOne(tag, goos, goarch) {
  const name = assetName(goos, goarch);
  const dest = path.join(BIN_DIR, name);

  // Resolve the download URL via the GitHub release asset API
  // (follows redirects to the signed S3 URL).
  const apiUrl = `https://github.com/${REPO}/releases/download/${encodeURIComponent(tag)}/${encodeURIComponent(name)}`;

  process.stdout.write(`  fetching ${goos}/${goarch} ... `);
  try {
    const stream = await fetch(apiUrl);
    const chunks = [];
    for await (const chunk of stream) {
      chunks.push(chunk);
    }
    const buf = Buffer.concat(chunks);
    if (buf.length === 0) {
      throw new Error('empty response');
    }
    fs.writeFileSync(dest, buf);
    if (goos !== 'windows') {
      fs.chmodSync(dest, 0o755);
    }
    console.log(`OK (${(buf.length / 1024 / 1024).toFixed(1)} MB)`);
    return true;
  } catch (err) {
    console.log(`FAILED (${err.message})`);
    return false;
  }
}

async function main() {
  const devMode = process.argv.includes('--dev');
  const tag = `v${readBundledVersion()}`;

  if (!fs.existsSync(BIN_DIR)) {
    fs.mkdirSync(BIN_DIR, { recursive: true });
  }

  const targets = devMode
    ? [currentPlatform()]
    : [
        { goos: 'linux', goarch: 'amd64' },
        { goos: 'linux', goarch: 'arm64' },
        { goos: 'darwin', goarch: 'amd64' },
        { goos: 'darwin', goarch: 'arm64' },
        { goos: 'windows', goarch: 'amd64' },
        { goos: 'windows', goarch: 'arm64' },
      ];

  console.log(`Downloading ctx binaries for tag ${tag}`);
  console.log(`  mode: ${devMode ? 'dev (current platform only)' : 'all platforms'}`);

  let failures = 0;
  for (const { goos, goarch } of targets) {
    const ok = await downloadOne(tag, goos, goarch);
    if (!ok) failures++;
  }

  if (failures > 0) {
    console.error(`\n${failures} download(s) failed.`);
    if (devMode) {
      console.error('Hint: for local dev, you can build the binary yourself:');
      console.error('  cd <ctx repo root> && go build -o vscode/bin/<asset> ./cmd/ctx');
    } else {
      console.error('Hint: the release tag may not exist yet. Create it with:');
      console.error('  git tag v<version> && git push --tags');
    }
    process.exit(1);
  }

  console.log(`\nDone. Binaries in ${BIN_DIR}`);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
