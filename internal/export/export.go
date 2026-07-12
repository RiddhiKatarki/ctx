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
	OutputPath       string
	ProjectName      string
	WorkingDir       string
	GitProvider      git.GitProvider
	PromptProvider   providers.PromptProvider
	SummaryProvider  summary.SummaryProvider
	ExtraFiles       []string
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

// Result holds the outcome of an export operation.
type Result struct {
	OutputPath string
	FileCount  int
	DiffSize   int
	BundleSize int64
	Skipped    []string
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

	fmt.Printf("✓ Detected Git repository\n")
	fmt.Printf("  Root: %s\n", repoRoot)

	gitMeta, err := cfg.GitProvider.Metadata()
	if err != nil {
		return nil, fmt.Errorf("failed to collect git metadata: %w", err)
	}

	statusStr := "clean"
	if gitMeta.Dirty {
		statusStr = "dirty"
	}
	fmt.Printf("  Branch: %s\n", gitMeta.CurrentBranch)
	fmt.Printf("  HEAD:   %s\n", gitMeta.HeadCommit[:min(7, len(gitMeta.HeadCommit))])
	fmt.Printf("  Status: %s\n", statusStr)

	prompts, err := cfg.PromptProvider.History()
	if err != nil {
		return nil, fmt.Errorf("failed to collect prompt history: %w", err)
	}
	fmt.Printf("✓ Collected %d prompt entries\n", len(prompts))

	modifiedFiles, err := cfg.GitProvider.ModifiedFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to get modified files: %w", err)
	}

	allFiles := mergeFiles(modifiedFiles, cfg.ExtraFiles)
	filtered, skipped := filterSecrets(allFiles)

	if len(skipped) > 0 {
		fmt.Printf("✓ Excluded %d file(s) matching secret patterns:\n", len(skipped))
		for _, s := range skipped {
			fmt.Printf("  - %s\n", s)
		}
	}

	fmt.Printf("✓ Collected %d file(s)\n", len(filtered))

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

	fmt.Printf("✓ Built snapshot\n")

	summ, err := cfg.SummaryProvider.Summarize(snapshot)
	if err != nil {
		return nil, fmt.Errorf("failed to generate summary: %w", err)
	}
	fmt.Printf("✓ Generated summary (%s)\n", providerName(cfg.SummaryProvider))

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

	fmt.Printf("\n✓ Bundle written: %s (%s)\n", outputPath, formatSize(bundleSize))
	fmt.Printf("  Sections: %s\n", strings.Join(schema.RequiredFiles, ", "))

	return &Result{
		OutputPath: outputPath,
		FileCount:  len(filtered),
		DiffSize:   len(diff),
		BundleSize: bundleSize,
		Skipped:    skipped,
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
