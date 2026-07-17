const esbuild = require('esbuild');

const isProd = process.argv.includes('--production');
const isWatch = process.argv.includes('--watch');
const sourcemap = process.argv.includes('--sourcemap');

/** @type {esbuild.BuildOptions} */
const options = {
  entryPoints: ['src/extension.ts'],
  bundle: true,
  format: 'cjs',
  platform: 'node',
  target: 'node18',
  outfile: 'dist/extension.js',
  external: ['vscode'],
  minify: isProd,
  sourcemap: sourcemap,
  logLevel: 'info',
  legalComments: 'none',
};

async function main() {
  if (isWatch) {
    const ctx = await esbuild.context(options);
    await ctx.watch();
    console.log('[esbuild] watching...');
  } else {
    await esbuild.build(options);
    console.log('[esbuild] build complete');
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
