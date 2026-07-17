import { spawn, type ChildProcess } from 'child_process';
import { createInterface } from 'readline';
import * as vscode from 'vscode';
import type {
  ErrorEnvelope,
  StreamEvent,
  VersionInfo,
} from '../types';

/**
 * Exit-code contract from internal/clierr/clierr.go:
 *   0 = success
 *   1 = user_error     (warning)
 *   2 = system_error   (error + offer logs)
 *   3 = invalid_bundle (error, bundle-specific copy)
 */
export const EXIT_CODES = {
  SUCCESS: 0,
  USER: 1,
  SYSTEM: 2,
  BUNDLE: 3,
} as const;

/**
 * Error class thrown by all client methods on failure. Carries the
 * parsed ErrorEnvelope (when --json was used) or a synthetic one.
 */
export class CtxError extends Error {
  constructor(
    message: string,
    public readonly envelope: ErrorEnvelope,
    public readonly exitCode: number | null,
    public readonly stderr: string,
    cause?: unknown,
  ) {
    super(message);
    this.name = 'CtxError';
    if (cause !== undefined) {
      // Node 22+: Error supports a cause option.
      (this as { cause?: unknown }).cause = cause;
    }
  }

  get code(): ErrorEnvelope['error']['code'] {
    return this.envelope.error.code;
  }
}

/**
 * Wraps the ctx binary. Always invokes with --json or --stream so we
 * parse structured output — never human-formatted text.
 *
 * Three modes of use:
 *   - version()          : runs `ctx version --json`
 *   - runJSON<T>(args)   : single-result commands (info, list, inspect)
 *   - runStream(args, cb): long-running commands with progress events
 *
 * Optionally pass options.cwd to spawn the child with that working
 * directory (required for `export`, which uses os.Getwd()).
 */
export interface RunOptions {
  cwd?: string;
}

export class CtxClient {
  constructor(
    private readonly binaryPath: string,
    private readonly outputChannel?: vscode.OutputChannel,
  ) {}

  /** Runs `ctx version --json`. */
  async version(): Promise<VersionInfo> {
    return this.runJSON<VersionInfo>(['version', '--json']);
  }

  /**
   * Runs ctx with --json appended; resolves with the single parsed
   * result object emitted on stdout. Rejects with CtxError on any
   * non-zero exit.
   */
  async runJSON<T>(args: string[], opts?: RunOptions): Promise<T> {
    const fullArgs = withJSON(args);
    const { stdout, stderr, code } = await this.exec(fullArgs, opts);

    if (code === EXIT_CODES.SUCCESS) {
      try {
        return JSON.parse(stdout) as T;
      } catch (err) {
        throw this.parseError(
          'ctx emitted malformed JSON',
          stdout,
          stderr,
          code,
          err,
        );
      }
    }

    throw this.parseError('ctx failed', stdout, stderr, code);
  }

  /**
   * Runs ctx with --stream appended; emits each NDJSON event via
   * onEvent. Resolves with the final `complete` event payload.
   * Rejects with CtxError on non-zero exit or malformed stream.
   *
   * Pass a CancellationToken to terminate the child process early;
   * the returned promise rejects with a CtxError whose code is
   * 'user_error' and message 'cancelled'.
   */
  async runStream<T = unknown>(
    args: string[],
    onEvent: (event: StreamEvent) => void,
    token?: vscode.CancellationToken,
    opts?: RunOptions,
  ): Promise<T> {
    const fullArgs = withStream(args);
    const child = this.spawn(fullArgs, opts);

    let stdoutBuffer = '';
    let stderrBuffer = '';
    let exitCode: number | null = null;
    let cancelled = false;

    const cancelSub = token?.onCancellationRequested(() => {
      cancelled = true;
      try {
        child.kill('SIGTERM');
      } catch {
        // Best effort; the child may already be dead.
      }
    });

    if (!child.stdout) {
      return Promise.reject(this.spawnError('ctx produced no stdout stream'));
    }
    const rl = createInterface({ input: child.stdout.setEncoding('utf8') });
    let lastEvent: StreamEvent | undefined;

    return new Promise<T>((resolve, reject) => {
      rl.on('line', (line) => {
        stdoutBuffer += line + '\n';
        const trimmed = line.trim();
        if (trimmed === '') {
          return;
        }
        let evt: StreamEvent;
        try {
          evt = JSON.parse(trimmed) as StreamEvent;
        } catch (err) {
          reject(
            this.parseError('ctx emitted a malformed NDJSON line', line, stderrBuffer, exitCode, err),
          );
          return;
        }
        if (evt.event === 'complete') {
          lastEvent = evt;
          return;
        }
        try {
          onEvent(evt);
        } catch (callbackErr) {
          // Don't let a buggy callback abort the whole pipeline silently.
          this.logError('onEvent threw', callbackErr);
        }
      });

      child.stderr?.setEncoding('utf8');
      child.stderr?.on('data', (chunk: string) => {
        stderrBuffer += chunk;
      });

      child.on('error', (err) => {
        cancelSub?.dispose();
        reject(new CtxError(
          `failed to spawn ctx: ${err.message}`,
          { error: { code: 'system_error', message: err.message } },
          null,
          stderrBuffer,
          err,
        ));
      });

      child.on('close', (code) => {
        cancelSub?.dispose();
        exitCode = code;

        if (cancelled) {
          reject(new CtxError(
            'cancelled',
            { error: { code: 'user_error', message: 'cancelled' } },
            code,
            stderrBuffer,
          ));
          return;
        }

        if (code === EXIT_CODES.SUCCESS && lastEvent) {
          resolve(lastEvent.data as T);
          return;
        }

        if (code === EXIT_CODES.SUCCESS) {
          // Stream ended without a `complete` event — malformed.
          reject(this.parseError(
            'ctx stream ended without a complete event',
            stdoutBuffer,
            stderrBuffer,
            code,
          ));
          return;
        }

        // Non-zero exit. Try to parse the last error event from stderr,
        // otherwise synthesize an envelope from the exit code.
        reject(this.parseError('ctx failed', stdoutBuffer, stderrBuffer, code));
      });
    });
  }

  // ---- internals --------------------------------------------------------

  private spawn(args: string[], opts?: RunOptions): ChildProcess {
    this.log(`spawn: ${this.binaryPath} ${args.join(' ')}${opts?.cwd ? ` (cwd: ${opts.cwd})` : ''}`);
    return spawn(this.binaryPath, args, {
      stdio: ['ignore', 'pipe', 'pipe'],
      windowsHide: true,
      cwd: opts?.cwd,
    });
  }

  private spawnError(message: string): CtxError {
    return new CtxError(
      message,
      { error: { code: 'system_error', message } },
      null,
      '',
    );
  }

  private exec(args: string[], opts?: RunOptions): Promise<{ stdout: string; stderr: string; code: number | null }> {
    const child = this.spawn(args, opts);
    let stdout = '';
    let stderr = '';

    return new Promise((resolve, reject) => {
      child.stdout?.setEncoding('utf8');
      child.stdout?.on('data', (chunk: string) => {
        stdout += chunk;
      });
      child.stderr?.setEncoding('utf8');
      child.stderr?.on('data', (chunk: string) => {
        stderr += chunk;
      });
      child.on('error', (err) => {
        reject(new CtxError(
          `failed to spawn ctx: ${err.message}`,
          { error: { code: 'system_error', message: err.message } },
          null,
          stderr,
          err,
        ));
      });
      child.on('close', (code) => {
        resolve({ stdout, stderr, code: code ?? null });
      });
    });
  }

  private parseError(
    fallbackMessage: string,
    _stdout: string,
    stderr: string,
    exitCode: number | null,
    cause?: unknown,
  ): CtxError {
    // The ctx binary writes its ErrorEnvelope to stderr when --json
    // is set (cmd/ctx/main.go:emitJSONError).
    const stderrTrimmed = stderr.trim();
    if (stderrTrimmed.startsWith('{')) {
      try {
        const envelope = JSON.parse(stderrTrimmed) as ErrorEnvelope;
        if (envelope?.error?.code && envelope?.error?.message) {
          return new CtxError(
            envelope.error.message,
            envelope,
            exitCode,
            stderr,
            cause,
          );
        }
      } catch {
        // fall through to synthesis
      }
    }

    // Synthesize from exit code.
    const code = exitCodeToCode(exitCode);
    const message = stderrTrimmed || fallbackMessage;
    return new CtxError(
      message,
      { error: { code, message } },
      exitCode,
      stderr,
      cause,
    );
  }

  private log(msg: string): void {
    this.outputChannel?.appendLine(`[client] ${msg}`);
  }

  private logError(prefix: string, err: unknown): void {
    const m = err instanceof Error ? `${err.message}\n${err.stack ?? ''}` : String(err);
    this.outputChannel?.appendLine(`[client] ${prefix}: ${m}`);
  }
}

// ---- helpers --------------------------------------------------------------

function withJSON(args: string[]): string[] {
  return args.includes('--json') ? args : [...args, '--json'];
}

function withStream(args: string[]): string[] {
  return args.includes('--stream') ? args : [...args, '--stream'];
}

function exitCodeToCode(exitCode: number | null): ErrorEnvelope['error']['code'] {
  switch (exitCode) {
    case EXIT_CODES.USER:
      return 'user_error';
    case EXIT_CODES.BUNDLE:
      return 'invalid_bundle';
    case EXIT_CODES.SYSTEM:
    default:
      return 'system_error';
  }
}
