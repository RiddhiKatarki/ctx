// TypeScript mirrors of the Go structs that the ctx binary emits via
// --json / --stream. Field names match the Go json tags verbatim.
// Source references are to /internal/**/*.go.

// --- pkg/types/types.go ----------------------------------------------------

export interface Manifest {
  version: number;
  created_at: string;
  tool: string;
  includes_contents?: boolean;
}

export interface Metadata {
  project_name: string;
  branch: string;
  created_at: string;
  generator: string;
  repository_root: string;
  os: string;
}

export interface GitMetadata {
  current_branch: string;
  head_commit: string;
  dirty: boolean;
  remote_url?: string;
  current_tag?: string;
}

export type PromptRole = 'user' | 'assistant' | 'system';

export interface Prompt {
  role: PromptRole;
  content: string;
}

export interface Summary {
  current_objective: string;
  completed_work: string;
  remaining_tasks: string;
  known_bugs: string;
  architecture_decisions: string;
  files_to_read_first: string;
  previous_failed_approaches: string;
  suggested_next_prompt: string;
  estimated_reading_time: string;
}

// --- internal/clierr/clierr.go --------------------------------------------

export type ErrorCode = 'user_error' | 'system_error' | 'invalid_bundle';

export interface ErrorEnvelope {
  error: {
    code: ErrorCode;
    message: string;
    cause?: string;
  };
}

// --- internal/reporter/reporter.go (NDJSON events) ------------------------

export interface StreamEvent {
  event: string;
  data: Record<string, unknown> | null;
}

// --- internal/discovery/discovery.go --------------------------------------

export interface BundleEntry {
  path: string;
  name: string;
  size: number;
  manifest_version: number;
  tool: string;
  created_at: string;
  project_name: string;
  branch: string;
  dirty: boolean;
  file_count: number;
  has_diff: boolean;
  head_commit: string;
}

export interface ListResult {
  directory: string;
  count: number;
  bundles: BundleEntry[];
}

export interface InfoResult {
  path: string;
  size: number;
  manifest: Record<string, unknown>;
  metadata: Record<string, unknown>;
  git: Record<string, unknown>;
  files: string[];
  file_count: number;
  summary_length: number;
  has_diff: boolean;
  diff_size: number;
  valid: boolean;
}

// --- internal/inspect/inspect.go ------------------------------------------

export interface InspectResult {
  path: string;
  manifest: Record<string, unknown>;
  metadata: Record<string, unknown>;
  files: string[];
  file_count: number;
  summary_sections: Record<string, string>;
  valid: boolean;
}

// --- internal/export/export.go --------------------------------------------

export interface SecretFinding {
  rule: string;
  severity: string;
  match: string;
  preview: string;
  line: number;
  column: number;
  source?: string;
}

export interface ExportResult {
  path: string;
  project_name: string;
  branch: string;
  repository_root: string;
  file_count: number;
  prompt_count: number;
  diff_size: number;
  bundle_size: number;
  skipped: string[];
  summary_provider: string;
  head_commit?: string;
  dirty: boolean;
  secrets?: SecretFinding[];
  secret_scan_enabled: boolean;
  includes_contents?: boolean;
  contents_count?: number;
  contents_skipped?: string[];
}

// --- internal/import/import.go --------------------------------------------

export interface ImportResult {
  path: string;
  manifest_version: number;
  tool: string;
  project_name: string;
  branch: string;
  created_at: string;
  generator: string;
  repository_root: string;
  os: string;
  head_commit: string;
  dirty: boolean;
  remote_url?: string;
  current_tag?: string;
  file_count: number;
  prompt_count: number;
  has_diff: boolean;
  diff_size: number;
  files: string[];
  extracted_to?: string;
  valid: boolean;
  includes_contents: boolean;
  contents_count: number;
}

// --- cmd/ctx/main.go (ctx version --json) ---------------------------------

export interface VersionInfo {
  version: string;
  bundle_format: number;
  binary_os: string;
  binary_arch: string;
}
