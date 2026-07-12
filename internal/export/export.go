package export

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/RiddhiKatarki/ctx/internal/archive"
	"github.com/RiddhiKatarki/ctx/internal/bundle"
	"github.com/RiddhiKatarki/ctx/internal/git"
	"github.com/RiddhiKatarki/ctx/internal/providers"
	"github.com/RiddhiKatarki/ctx/internal/schema"
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

	// JSONOutput, when true, suppresses human-readable prints
	// and the caller is expected to encode the Result as JSON.
	// Error formatting remains the caller's responsibility.
	JSONOutput bool
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
	OutputPath      string   `json:"path"`
	ProjectName     string   `json:"project_name"`
	Branch          string   `json:"branch"`
	RepoRoot        string   `json:"repository_root"`
	FileCount       int      `json:"file_count"`
	PromptCount     int      `json:"prompt_count"`
	DiffSize        int      `json:"diff_size"`
	BundleSize      int64    `json:"bundle_size"`
	Skipped         []string `json:"skipped"`
	SummaryProvider string   `json:"summary_provider"`
	Commit         string   `json:"head_commit,omitempty"`
	Dirty          bool     `json:"dirty"`
}

// human prints only when JSONOutput is false.
func (cfg Config) human(format string, a ...any) {
	if cfg.JSONOutput {
		return
	}
	fmt.Printf(format, a...)
}

// Run executes the full export flow:
// detect git → read metadata → read prompts → collect files →
// read diff → build snapshot → generate summary → create archive.
func Run(cfg Config) (*Result, error) {
	if cfg.GitProvider == nil {
		cfg.GitProvider = git.NewCLIGitProvider(cfg.WorkingDir)
	}
	if cfg.PromptProvider == nil {
		cfg.PromptProvider = providers.NewPromptProvider(providers.Options{})
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

	if !cfg.GitProvider.IsRepository() {
		return nil, fmt.Errorf("not a git repository (or any of the parent directories): .git")
	}

	repoRoot, err := cfg.GitProvider.RepositoryRoot()
	if err != nil {
		return nil, fmt.Errorf("failed to determine repository root: %w", err)
	}

	cfg.human("✓ Detected Git repository\n")
	cfg.human("  Root: %s\n", repoRoot)

	gitMeta, err := cfg.GitProvider.Metadata()
	if err != nil {
		return nil, fmt.Errorf("failed to collect git metadata: %w", err)
	}

	statusStr := "clean"
	if gitMeta.Dirty {
		statusStr = "dirty"
	}
	cfg.human("  Branch: %s\n", gitMeta.CurrentBranch)
	cfg.human("  HEAD:   %s\n", gitMeta.HeadCommit[:min(7, len(gitMeta.HeadCommit))])
	cfg.human("  Status: %s\n", statusStr)

	prompts, err := cfg.PromptProvider.History()
	if err != nil {
		return nil, fmt.Errorf("failed to collect prompt history: %w", err)
	}
	cfg.human("✓ Collected %d prompt entries\n", len(prompts))

	modifiedFiles, err := cfg.GitProvider.ModifiedFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to get modified files: %w", err)
	}

	allFiles := mergeFiles(modifiedFiles, cfg.ExtraFiles)
	filtered, skipped := filterSecrets(allFiles)

	if len(skipped) > 0 {
		cfg.human("✓ Excluded %d file(s) matching secret patterns:\n", len(skipped))
		for _, s := range skipped {
			cfg.human("  - %s\n", s)
		}
	}

	cfg.human("✓ Collected %d file(s)\n", len(filtered))

	diffBytes, err := cfg.GitProvider.Diff()
	if err != nil {
		return nil, fmt.Errorf("failed to get git diff: %w", err)
	}
	diff := string(diffBytes)

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

	cfg.human("✓ Built snapshot\n")

	summ, err := cfg.SummaryProvider.Summarize(snapshot)
	if err != nil {
		return nil, fmt.Errorf("failed to generate summary: %w", err)
	}
	cfg.human("✓ Generated summary (%s)\n", providerName(cfg.SummaryProvider))

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

	cfg.human("\n✓ Bundle written: %s (%s)\n", outputPath, formatSize(bundleSize))
	cfg.human("  Sections: %s\n", strings.Join(schema.RequiredFiles, ", "))

	return &Result{
		OutputPath:      outputPath,
		ProjectName:    projectName,
		Branch:         gitMeta.CurrentBranch,
		RepoRoot:       repoRoot,
		FileCount:      len(filtered),
		PromptCount:    len(prompts),
		DiffSize:       len(diff),
		BundleSize:     bundleSize,
		Skipped:        skipped,
		SummaryProvider: providerName(cfg.SummaryProvider),
		Commit:         gitMeta.HeadCommit,
		Dirty:          gitMeta.Dirty,
	}, nil
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
