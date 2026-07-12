package importctx

import (
	"fmt"
	"strings"

	"github.com/context-handoff/ctx/internal/archive"
	"github.com/context-handoff/ctx/internal/bundle"
)

// Config holds options for the import operation.
type Config struct {
	// Path is the path to the .ctx bundle file.
	Path string
	// OutDir is the optional directory to extract files to.
	// If empty, the bundle is only validated and displayed.
	OutDir string
}

// Result holds the outcome of an import operation.
type Result struct {
	ProjectName string
	Branch      string
	FileCount   int
	PromptCount int
	HasDiff     bool
	ExtractedTo string
}

// Run executes the import flow:
// validate archive → extract → read manifest → display summary.
func Run(cfg Config) (*Result, error) {
	if cfg.Path == "" {
		return nil, fmt.Errorf("no bundle path specified")
	}

	if !archive.IsZipFile(cfg.Path) {
		return nil, fmt.Errorf("%s is not a valid .ctx archive (not a ZIP file)", cfg.Path)
	}

	fmt.Printf("✓ Validating archive: %s\n", cfg.Path)

	files, err := archive.Extract(cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to extract archive: %w", err)
	}

	fmt.Printf("✓ Archive extracted (%d files)\n", len(files))

	b, err := bundle.Deserialize(files)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize bundle: %w", err)
	}

	fmt.Printf("✓ Manifest validated (version %d, tool: %s)\n", b.Manifest.Version, b.Manifest.Tool)

	result := &Result{
		ProjectName: b.Metadata.ProjectName,
		Branch:      b.Metadata.Branch,
		FileCount:   len(b.Files),
		PromptCount: len(b.Prompts),
		HasDiff:     b.Diff != "",
	}

	if cfg.OutDir != "" {
		if err := archive.ExtractToDir(cfg.Path, cfg.OutDir); err != nil {
			return nil, fmt.Errorf("failed to extract to directory: %w", err)
		}
		result.ExtractedTo = cfg.OutDir
		fmt.Printf("✓ Files extracted to: %s\n", cfg.OutDir)
	}

	fmt.Println()
	fmt.Println("════════════════════════════════════════")
	fmt.Println("  Bundle Summary")
	fmt.Println("════════════════════════════════════════")
	fmt.Printf("  Project:       %s\n", b.Metadata.ProjectName)
	fmt.Printf("  Branch:        %s\n", b.Metadata.Branch)
	fmt.Printf("  Created:       %s\n", b.Metadata.CreatedAt.Format("2006-01-02 15:04:05 UTC"))
	fmt.Printf("  Generator:     %s\n", b.Metadata.Generator)
	fmt.Printf("  Repository:    %s\n", b.Metadata.RepositoryRoot)
	fmt.Printf("  OS:            %s\n", b.Metadata.OS)
	fmt.Println("──────────────────────────────────────────")
	fmt.Printf("  HEAD:          %s\n", b.Git.HeadCommit)
	fmt.Printf("  Dirty:         %t\n", b.Git.Dirty)
	if b.Git.RemoteURL != "" {
		fmt.Printf("  Remote:        %s\n", b.Git.RemoteURL)
	}
	if b.Git.CurrentTag != "" {
		fmt.Printf("  Tag:           %s\n", b.Git.CurrentTag)
	}
	fmt.Println("──────────────────────────────────────────")
	fmt.Printf("  Files:         %d\n", len(b.Files))
	fmt.Printf("  Prompts:       %d\n", len(b.Prompts))
	fmt.Printf("  Diff present:  %t\n", b.Diff != "")
	if b.Diff != "" {
		fmt.Printf("  Diff size:     %d bytes\n", len(b.Diff))
	}
	fmt.Println("════════════════════════════════════════")

	if len(b.Files) > 0 {
		fmt.Println()
		fmt.Println("Modified Files:")
		for _, f := range b.Files {
			fmt.Printf("  %s\n", f)
		}
	}

	if b.Diff != "" && cfg.OutDir != "" {
		fmt.Println()
		fmt.Printf("To apply the uncommitted changes:\n")
		fmt.Printf("  git apply %s/patch.diff\n", cfg.OutDir)
	}

	return result, nil
}

// QuickSummary returns a one-line summary string for display.
func QuickSummary(r *Result) string {
	parts := []string{
		fmt.Sprintf("project=%s", r.ProjectName),
		fmt.Sprintf("branch=%s", r.Branch),
		fmt.Sprintf("files=%d", r.FileCount),
		fmt.Sprintf("prompts=%d", r.PromptCount),
	}
	return strings.Join(parts, " ")
}
