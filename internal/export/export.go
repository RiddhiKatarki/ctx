package export

import (
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/RiddhiKatarki/ctx/internal/archive"
	"github.com/RiddhiKatarki/ctx/internal/bundle"
	"github.com/RiddhiKatarki/ctx/internal/git"
	"github.com/RiddhiKatarki/ctx/internal/providers"
	"github.com/RiddhiKatarki/ctx/internal/reporter"
	"github.com/RiddhiKatarki/ctx/internal/schema"
	"github.com/RiddhiKatarki/ctx/internal/secretscan"
	"github.com/RiddhiKatarki/ctx/internal/summary"
	"github.com/RiddhiKatarki/ctx/pkg/types"
)

// Config holds all dependencies and options for the export operation.
type Config struct {
	OutputPath      string
	ProjectName     string
	WorkingDir      string
	GitProvider     git.GitProvider
	PromptProvider  providers.PromptProvider
	SummaryProvider summary.SummaryProvider
	ExtraFiles      []string

	// Reporter controls progress output. Defaults to a HumanReporter
	// writing to stderr if nil. JSON / Stream reporters emit structured
	// events and a final result on Done().
	Reporter reporter.Reporter

	// JSONOutput is retained for backwards compatibility — when true
	// and Reporter is nil, a JSONReporter is constructed automatically.
	JSONOutput bool

	// DisableSecretScan short-circuits content scanning. Filename
	// patterns (.env, *.pem, etc.) still apply regardless.
	DisableSecretScan bool

	// IncludeSecrets, when true, preserves raw secret matches in the
	// bundle (only redacted previews are always safe). Defaults to false
	// — Phase 5 default excludes secret-bearing files and redacts the diff.
	IncludeSecrets bool

	// Scanner overrides the default secretscan.Scanner. Mostly useful
	// for testing or custom rule sets.
	Scanner *secretscan.Scanner
}

// secretPatterns are file patterns that are excluded from the bundle
// for security. This anticipates future security features without
// implementing full secret scanning in V1.
var secretPatterns = []string{
	".env",
	".env.local",
	"*.pem",
	"*.key",
	"id_rsa",
	"id_rsa.pub",
	"*.p12",
	"*.pfx",
}

// Result holds the outcome of an export operation. JSON tags enable
// machine-readable output when emitted with --json.
type Result struct {
	OutputPath       string                 `json:"path"`
	ProjectName      string                 `json:"project_name"`
	Branch           string                 `json:"branch"`
	RepoRoot         string                 `json:"repository_root"`
	FileCount        int                    `json:"file_count"`
	PromptCount      int                    `json:"prompt_count"`
	DiffSize         int                    `json:"diff_size"`
	BundleSize       int64                  `json:"bundle_size"`
	Skipped          []string               `json:"skipped"`
	SummaryProvider  string                 `json:"summary_provider"`
	Commit           string                 `json:"head_commit,omitempty"`
	Dirty            bool                   `json:"dirty"`
	Secrets          []secretscan.Finding   `json:"secrets,omitempty"`
	SecretScanEnabled bool                  `json:"secret_scan_enabled"`
}

// human prints only when the Reporter is a HumanReporter — this preserves
// the historical printf-path while still routing through Reporter so future
// per-event customisation stays consistent.
func (cfg Config) human(format string, a ...any) {
	r := cfg.rep()
	r.Info(format, a...)
}

// rep returns cfg.Reporter or a default HumanReporter on stdout.
func (cfg Config) rep() reporter.Reporter {
	if cfg.Reporter != nil {
		return cfg.Reporter
	}
	if cfg.JSONOutput {
		return defaultJSONReporter
	}
	return defaultHumanReporter
}

// defaults are process-wide fallbacks used when callers don't supply one.
var (
	defaultHumanReporter = reporter.NewHumanReporter(nil)
	defaultJSONReporter  = reporter.NewJSONReporter(nil)
)

// Run executes the full export flow:
// detect git → read metadata → read prompts → collect files →
// read diff → build snapshot → generate summary → create archive.
func Run(cfg Config) (*Result, error) {
	if cfg.GitProvider == nil {
		cfg.GitProvider = git.NewCLIGitProvider(cfg.WorkingDir)
	}
	if cfg.PromptProvider == nil {
		pp, err := providers.NewPromptProvider(providers.Options{Source: providers.SourceAuto})
		if err != nil {
			return nil, err
		}
		cfg.PromptProvider = pp
	}
	if cfg.SummaryProvider == nil {
		sp, err := summary.NewSummaryProvider(summary.Options{})
		if err != nil {
			return nil, err
		}
		cfg.SummaryProvider = sp
	}
	if cfg.OutputPath == "" {
		cfg.OutputPath = "project.ctx"
	}
	if cfg.Scanner == nil {
		cfg.Scanner = secretscan.New()
	}

	r := cfg.rep()
	r.Event("start", map[string]any{"output": cfg.OutputPath})
	r.Info("✓ Detected Git repository\n")

	if !cfg.GitProvider.IsRepository() {
		r.Event("error", map[string]any{"stage": "git_detect", "message": "not a git repository"})
		return nil, fmt.Errorf("not a git repository (or any of the parent directories): .git")
	}

	repoRoot, err := cfg.GitProvider.RepositoryRoot()
	if err != nil {
		return nil, fmt.Errorf("failed to determine repository root: %w", err)
	}

	r.Event("git_detected", map[string]any{"root": repoRoot})
	r.Info("  Root: %s\n", repoRoot)

	gitMeta, err := cfg.GitProvider.Metadata()
	if err != nil {
		return nil, fmt.Errorf("failed to collect git metadata: %w", err)
	}

	statusStr := "clean"
	if gitMeta.Dirty {
		statusStr = "dirty"
	}
	r.Event("git_metadata", map[string]any{
		"branch": gitMeta.CurrentBranch,
		"head":   gitMeta.HeadCommit,
		"dirty":  gitMeta.Dirty,
	})
	r.Info("  Branch: %s\n", gitMeta.CurrentBranch)
	r.Info("  HEAD:   %s\n", gitMeta.HeadCommit[:min(7, len(gitMeta.HeadCommit))])
	r.Info("  Status: %s\n", statusStr)

	prompts, err := cfg.PromptProvider.History()
	if err != nil {
		return nil, fmt.Errorf("failed to collect prompt history: %w", err)
	}
	r.Event("prompts_collected", map[string]any{"count": len(prompts)})
	r.Info("✓ Collected %d prompt entries\n", len(prompts))

	modifiedFiles, err := cfg.GitProvider.ModifiedFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to get modified files: %w", err)
	}

	allFiles := mergeFiles(modifiedFiles, cfg.ExtraFiles)

	// Apply .ctxignore if present in the working dir.
	igPath := filepath.Join(cfg.WorkingDir, ".ctxignore")
	ig, _ := secretscan.LoadIgnoreFile(igPath) // missing file is non-fatal
	if ig != nil && len(ig.Patterns()) > 0 {
		r.Event("ctxignore_loaded", map[string]any{"patterns": ig.Patterns()})
		kept := allFiles[:0]
		var ignored []string
		for _, f := range allFiles {
			if ig.Match(f) {
				ignored = append(ignored, f)
			} else {
				kept = append(kept, f)
			}
		}
		if len(ignored) > 0 {
			r.Info("✓ Excluded %d file(s) by .ctxignore:\n", len(ignored))
			for _, s := range ignored {
				r.Info("  - %s\n", s)
			}
		}
		allFiles = kept
	}

	filtered, skipped := filterSecrets(allFiles)

	if len(skipped) > 0 {
		r.Event("files_excluded", map[string]any{"count": len(skipped), "patterns": secretPatterns})
		r.Info("✓ Excluded %d file(s) matching secret patterns:\n", len(skipped))
		for _, s := range skipped {
			r.Info("  - %s\n", s)
		}
	}

	r.Event("files_collected", map[string]any{"count": len(filtered)})
	r.Info("✓ Collected %d file(s)\n", len(filtered))

	diffBytes, err := cfg.GitProvider.Diff()
	if err != nil {
		return nil, fmt.Errorf("failed to get git diff: %w", err)
	}
	diff := string(diffBytes)
	r.Event("diff_captured", map[string]any{"size": len(diff)})

	var findings []secretscan.Finding
	if !cfg.DisableSecretScan && cfg.Scanner != nil {
		findings = cfg.Scanner.Scan([]byte(diff), "patch.diff")
		if len(findings) > 0 {
			r.Event("secrets_found", map[string]any{
				"count":  len(findings),
				"by_rule": secretscan.Summarise(findings),
			})
			if !cfg.IncludeSecrets {
				diff = redactFindings(diff, findings)
				r.Info("✓ Detected %d secret(s) in diff (redacted):\n", len(findings))
				for _, f := range findings {
					r.Info("  - [%s] %s (line %d)\n", f.Severity, f.Rule, f.Line)
				}
			} else {
				r.Info("⚠ Detected %d secret(s) in diff (kept raw via --include-secrets):\n", len(findings))
				for _, f := range findings {
					r.Info("  - [%s] %s (line %d)\n", f.Severity, f.Rule, f.Line)
				}
			}
		}
	}

	projectName := cfg.ProjectName
	if projectName == "" {
		projectName = filepath.Base(repoRoot)
	}

	metadata := types.Metadata{
		ProjectName:    projectName,
		Branch:         gitMeta.CurrentBranch,
		CreatedAt:      time.Now().UTC(),
		Generator:      schema.ToolName,
		RepositoryRoot: repoRoot,
		OS:             runtimeOS(),
	}

	snapshot := types.Snapshot{
		Metadata: metadata,
		Git:      *gitMeta,
		Prompts:  prompts,
		Files:    filtered,
		Diff:     diff,
	}

	if skipped == nil {
		skipped = []string{}
	}

	r.Event("snapshot_built", nil)
	r.Info("✓ Built snapshot\n")

	r.Event("summary_start", map[string]any{"provider": providerName(cfg.SummaryProvider)})
	summ, err := cfg.SummaryProvider.Summarize(snapshot)
	if err != nil {
		return nil, fmt.Errorf("failed to generate summary: %w", err)
	}
	r.Event("summary_complete", map[string]any{"provider": providerName(cfg.SummaryProvider)})
	r.Info("✓ Generated summary (%s)\n", providerName(cfg.SummaryProvider))

	b := bundle.Build(snapshot, summ)

	serialized, err := b.Serialize()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize bundle: %w", err)
	}

	outputPath := archive.EnsureCtxExtension(cfg.OutputPath)
	if err := archive.Create(outputPath, serialized); err != nil {
		return nil, fmt.Errorf("failed to create archive: %w", err)
	}

	bundleSize, _ := archive.Size(outputPath)
	r.Event("bundle_written", map[string]any{"path": outputPath, "size": bundleSize})
	r.Info("\n✓ Bundle written: %s (%s)\n", outputPath, formatSize(bundleSize))
	r.Info("  Sections: %s\n", strings.Join(schema.RequiredFiles, ", "))

	// Findings exposed externally should always carry redacted previews,
	// never the raw Match value.
	safe := make([]secretscan.Finding, len(findings))
	for i, f := range findings {
		safe[i] = f
		if !cfg.IncludeSecrets {
			safe[i].Match = secretscan.RedactMatch(f.Match)
		}
	}

	result := &Result{
		OutputPath:        outputPath,
		ProjectName:       projectName,
		Branch:            gitMeta.CurrentBranch,
		RepoRoot:          repoRoot,
		FileCount:         len(filtered),
		PromptCount:       len(prompts),
		DiffSize:          len(diff),
		BundleSize:        bundleSize,
		Skipped:           skipped,
		SummaryProvider:   providerName(cfg.SummaryProvider),
		Commit:            gitMeta.HeadCommit,
		Dirty:             gitMeta.Dirty,
		Secrets:           safe,
		SecretScanEnabled: !cfg.DisableSecretScan,
	}

	r.Done(result)
	return result, nil
}

// redactFindings replaces secret matches in raw with [REDACTED:<rule>]
// markers. The rule used in the marker follows the order findings
// were discovered — earlier rules (more specific patterns) win over
// the high-entropy fallback when the same match text matches several.
func redactFindings(raw string, findings []secretscan.Finding) string {
	if len(findings) == 0 {
		return raw
	}

	type span struct {
		start, end int
		rule       string
	}

	// For each unique Match text, prefer the first rule that reported it.
	seen := make(map[string]bool)
	var spans []span
	for _, f := range findings {
		if seen[f.Match] {
			continue
		}
		seen[f.Match] = true
		from := 0
		for {
			pos := strings.Index(raw[from:], f.Match)
			if pos < 0 {
				break
			}
			abs := from + pos
			spans = append(spans, span{start: abs, end: abs + len(f.Match), rule: f.Rule})
			from = abs + len(f.Match)
		}
	}
	if len(spans) == 0 {
		return raw
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].start < spans[j].start })

	var b strings.Builder
	cursor := 0
	for _, s := range spans {
		b.WriteString(raw[cursor:s.start])
		b.WriteString("[REDACTED:" + s.rule + "]")
		cursor = s.end
	}
	b.WriteString(raw[cursor:])
	return b.String()
}

func mergeFiles(modified, extra []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, f := range modified {
		if !seen[f] {
			seen[f] = true
			result = append(result, f)
		}
	}
	for _, f := range extra {
		if !seen[f] {
			seen[f] = true
			result = append(result, f)
		}
	}
	return result
}

func filterSecrets(files []string) (kept, skipped []string) {
	for _, f := range files {
		if isSecretFile(f) {
			skipped = append(skipped, f)
		} else {
			kept = append(kept, f)
		}
	}
	return kept, skipped
}

func isSecretFile(path string) bool {
	base := filepath.Base(path)
	for _, pattern := range secretPatterns {
		matched, err := filepath.Match(pattern, base)
		if err == nil && matched {
			return true
		}
	}
	return false
}

func runtimeOS() string {
	return fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
}

func providerName(p summary.SummaryProvider) string {
	switch p.(type) {
	case *summary.TemplateProvider:
		return "template"
	case *summary.OpenAIProvider:
		return "openai-compatible"
	default:
		return "unknown"
	}
}

func formatSize(bytes int64) string {
	switch {
	case bytes < 1024:
		return fmt.Sprintf("%d B", bytes)
	case bytes < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
