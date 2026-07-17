import { describe, it, expect } from 'vitest';
import { readFileSync } from 'fs';
import { join } from 'path';

const PKG = JSON.parse(
  readFileSync(join(__dirname, '..', '..', 'package.json'), 'utf8'),
);

describe('package.json manifest', () => {
  it('declares the main entrypoint compiled by esbuild', () => {
    expect(PKG.main).toBe('./dist/extension.js');
  });

  it('targets a supported VS Code engine range', () => {
    expect(PKG.engines.vscode).toMatch(/^\^1\.\d+\.\d+$/);
  });

  it('declares every command with a matching contributes entry', () => {
    const commandIds = PKG.contributes.commands.map((c: { command: string }) => c.command);
    expect(commandIds).toContain('ctx.export');
    expect(commandIds).toContain('ctx.import');
    expect(commandIds).toContain('ctx.inspect');
    expect(commandIds).toContain('ctx.info');
    expect(commandIds).toContain('ctx.applyPatch');
    expect(commandIds).toContain('ctx.list.refresh');
  });

  it('declares the activity-bar view container with the bundles view', () => {
    const containers = PKG.contributes.viewsContainers.activitybar;
    expect(containers).toHaveLength(1);
    expect(containers[0].id).toBe('ctx');

    const views = PKG.contributes.views.ctx;
    expect(views).toHaveLength(1);
    expect(views[0].id).toBe('ctx.bundles');
  });

  it('declares explorer context menus only for .ctx files', () => {
    const entries = PKG.contributes.menus['explorer/context'];
    expect(entries.length).toBeGreaterThan(0);
    for (const e of entries) {
      expect(e.when).toContain('.ctx');
    }
  });

  it('declares all configuration keys with defaults', () => {
    const keys = Object.keys(PKG.contributes.configuration.properties);
    expect(keys).toContain('ctx.summaryProvider');
    expect(keys).toContain('ctx.openaiBaseUrl');
    expect(keys).toContain('ctx.openaiModel');
    expect(keys).toContain('ctx.defaultOutputName');
    expect(keys).toContain('ctx.defaultOutdir');
    expect(keys).toContain('ctx.secretScanEnabled');
    expect(keys).toContain('ctx.includeContents');
    expect(keys).toContain('ctx.contentsThreshold');
    expect(keys).toContain('ctx.showNotifications');
  });
});
