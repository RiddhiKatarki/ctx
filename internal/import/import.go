package importctx

import (
	"fmt"
	"strings"

	"github.com/RiddhiKatarki/ctx/internal/archive"
	"github.com/RiddhiKatarki/ctx/internal/bundle"
)

// Config holds options for the import operation.
type Config struct {
	// Path is the path to the .ctx bundle file.
	Path string
	// OutDir is the optional directory to extract files to.
	// If empty, the bundle is only validated and displayed.
	OutDir string

	// JSONOutput, when true, suppresses human-readable prints
	// and the caller is expected to encode the Result as JSON.
	JSONOutput bool
}

// Result holds the outcome of an import operation. JSON tags enable
// machine-readable output when emitted with --json.
type Result struct {
	Path            string   `json:"path"`
	ManifestVersion int      `json:"manifest_version"`
	Tool            string   `json:"tool"`
	ProjectName     string   `json:"project_name"`
	Branch          string   `json:"branch"`
	CreatedAt       string   `json:"created_at"`
	Generator       string   `json:"generator"`
	RepositoryRoot  string   `json:"repository_root"`
	OS              string   `json:"os"`
	Commit          string   `json:"head_commit"`
	Dirty           bool     `json:"dirty"`
	RemoteURL       string   `json:"remote_url,omitempty"`
	CurrentTag      string   `json:"current_tag,omitempty"`
	FileCount       int      `json:"file_count"`
	PromptCount     int      `json:"prompt_count"`
	HasDiff         bool     `json:"has_diff"`
	DiffSize        int      `json:"diff_size"`
	Files           []string `json:"files"`
	ExtractedTo     string   `json:"extracted_to,omitempty"`
	Valid           bool     `json:"valid"`
}

func (cfg Config) human(format string, a ...any) {
	if cfg.JSONOutput {
		return
	}
	fmt.Printf(format, a...)
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

	cfg.human("✓ Validating archive: %s\n", cfg.Path)

	files, err := archive.Extract(cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to extract archive: %w", err)
	}

	cfg.human("✓ Archive extracted (%d files)\n", len(files))

	b, err := bundle.Deserialize(files)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize bundle: %w", err)
	}

	cfg.human("✓ Manifest validated (version %d, tool: %s)\n", b.Manifest.Version, b.Manifest.Tool)

	result := &Result{
		Path:            cfg.Path,
		ManifestVersion: b.Manifest.Version,
		Tool:            b.Manifest.Tool,
		ProjectName:     b.Metadata.ProjectName,
		Branch:          b.Metadata.Branch,
		CreatedAt:       b.Metadata.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		Generator:       b.Metadata.Generator,
		RepositoryRoot:  b.Metadata.RepositoryRoot,
		OS:              b.Metadata.OS,
		Commit:          b.Git.HeadCommit,
		Dirty:           b.Git.Dirty,
		RemoteURL:       b.Git.RemoteURL,
		CurrentTag:      b.Git.CurrentTag,
		FileCount:       len(b.Files),
		PromptCount:     len(b.Prompts),
		HasDiff:         b.Diff != "",
		DiffSize:        len(b.Diff),
		Files:           b.Files,
		Valid:           true,
	}

	if result.Files == nil {
		result.Files = []string{}
	}

	if cfg.OutDir != "" {
		if err := archive.ExtractToDir(cfg.Path, cfg.OutDir); err != nil {
			return nil, fmt.Errorf("failed to extract to directory: %w", err)
		}
		result.ExtractedTo = cfg.OutDir
		cfg.human("✓ Files extracted to: %s\n", cfg.OutDir)
	}

	cfg.human("\n════════════════════════════════════════\n")
	cfg.human("  Bundle Summary\n")
	cfg.human("════════════════════════════════════════\n")
	cfg.human("  Project:       %s\n", b.Metadata.ProjectName)
	cfg.human("  Branch:        %s\n", b.Metadata.Branch)
	cfg.human("  Created:       %s\n", b.Metadata.CreatedAt.Format("2006-01-02 15:04:05 UTC"))
	cfg.human("  Generator:     %s\n", b.Metadata.Generator)
	cfg.human("  Repository:    %s\n", b.Metadata.RepositoryRoot)
	cfg.human("  OS:            %s\n", b.Metadata.OS)
	cfg.human("──────────────────────────────────────────\n")
	cfg.human("  HEAD:          %s\n", b.Git.HeadCommit)
	cfg.human("  Dirty:         %t\n", b.Git.Dirty)
	if b.Git.RemoteURL != "" {
		cfg.human("  Remote:        %s\n", b.Git.RemoteURL)
	}
	if b.Git.CurrentTag != "" {
		cfg.human("  Tag:           %s\n", b.Git.CurrentTag)
	}
	cfg.human("──────────────────────────────────────────\n")
	cfg.human("  Files:         %d\n", len(b.Files))
	cfg.human("  Prompts:       %d\n", len(b.Prompts))
	cfg.human("  Diff present:  %t\n", b.Diff != "")
	if b.Diff != "" {
		cfg.human("  Diff size:     %d bytes\n", len(b.Diff))
	}
	cfg.human("════════════════════════════════════════\n")

	if len(b.Files) > 0 {
		cfg.human("\nModified Files:\n")
		for _, f := range b.Files {
			cfg.human("  %s\n", f)
		}
	}

	if b.Diff != "" && cfg.OutDir != "" {
		cfg.human("\nTo apply the uncommitted changes:\n")
		cfg.human("  git apply %s/patch.diff\n", cfg.OutDir)
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
