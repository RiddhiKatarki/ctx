export const EXTENSION_ID = 'riddhikatarki.ctx';

export const MIN_BINARY_VERSION = '2.1.0';
export const BUNDLED_BINARY_VERSION = '2.1.0';

export const COMMANDS = {
  exportCtx: 'ctx.export',
  importCtx: 'ctx.import',
  inspect: 'ctx.inspect',
  info: 'ctx.info',
  applyPatch: 'ctx.applyPatch',
  listRefresh: 'ctx.list.refresh',
} as const;

export const VIEWS = {
  bundles: 'ctx.bundles',
} as const;

export const CONFIG = {
  summaryProvider: 'ctx.summaryProvider',
  openaiBaseUrl: 'ctx.openaiBaseUrl',
  openaiModel: 'ctx.openaiModel',
  defaultOutputName: 'ctx.defaultOutputName',
  defaultOutdir: 'ctx.defaultOutdir',
  secretScanEnabled: 'ctx.secretScanEnabled',
  includeContents: 'ctx.includeContents',
  contentsThreshold: 'ctx.contentsThreshold',
  showNotifications: 'ctx.showNotifications',
} as const;

export const SECRET_KEYS = {
  openaiApiKey: 'openaiApiKey',
} as const;

export const OUTPUT_CHANNEL_NAME = 'ctx';
