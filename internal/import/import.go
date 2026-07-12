package importctx

import (
	"fmt"
	"strings"

	"github.com/RiddhiKatarki/ctx/internal/archive"
	"github.com/RiddhiKatarki/ctx/internal/bundle"
	"github.com/RiddhiKatarki/ctx/internal/reporter"
)

// Config holds options for the import operation.
type Config struct {
	// Path is the path to the .ctx bundle file.
	Path string
	// OutDir is the optional directory to extract files to.
	// If empty, the bundle is only validated and displayed.
	OutDir string

	// Reporter controls progress output for stream/json modes.
	Reporter reporter.Reporter

	// JSONOutput retained for backwards compat — auto-creates JSONReporter.
	JSONOutput bool
}

// Result holds the outcome of an import operation. JSON tags enable
// machine-readable output when emitted with --json.
type Result struct {
	Path             string   `json:"path"`
	ManifestVersion  int      `json:"manifest_version"`
	Tool             string   `json:"tool"`
	ProjectName      string   `json:"project_name"`
	Branch           string   `json:"branch"`
	CreatedAt        string   `json:"created_at"`
	Generator        string   `json:"generator"`
	RepositoryRoot   string   `json:"repository_root"`
	OS               string   `json:"os"`
	Commit           string   `json:"head_commit"`
	Dirty            bool     `json:"dirty"`
	RemoteURL        string   `json:"remote_url,omitempty"`
	CurrentTag       string   `json:"current_tag,omitempty"`
	FileCount        int      `json:"file_count"`
	PromptCount      int      `json:"prompt_count"`
	HasDiff          bool     `json:"has_diff"`
	DiffSize         int      `json:"diff_size"`
	Files            []string `json:"files"`
	ExtractedTo      string   `json:"extracted_to,omitempty"`
	Valid            bool     `json:"valid"`
	IncludesContents bool     `json:"includes_contents"`
	ContentsCount    int      `json:"contents_count"`
}

var (
	defaultHumanReporter = reporter.NewHumanReporter(nil)
	defaultJSONReporter  = reporter.NewJSONReporter(nil)
)

func (cfg Config) rep() reporter.Reporter {
	if cfg.Reporter != nil {
		return cfg.Reporter
	}
	if cfg.JSONOutput {
		return defaultJSONReporter
	}
	return defaultHumanReporter
}

// Run executes the import flow:
// validate archive → extract → read manifest → display summary.
func Run(cfg Config) (*Result, error) {
	r := cfg.rep()
	r.Event("start", map[string]any{"path": cfg.Path})

	if cfg.Path == "" {
		r.Event("error", map[string]any{"stage": "input", "message": "no bundle path specified"})
		return nil, fmt.Errorf("no bundle path specified")
	}

	if !archive.IsZipFile(cfg.Path) {
		r.Event("error", map[string]any{"stage": "validate", "message": "not a valid .ctx archive"})
		return nil, fmt.Errorf("%s is not a valid .ctx archive (not a ZIP file)", cfg.Path)
	}

	r.Info("✓ Validating archive: %s\n", cfg.Path)

	files, err := archive.Extract(cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to extract archive: %w", err)
	}

	r.Event("extracted", map[string]any{"count": len(files)})
	r.Info("✓ Archive extracted (%d files)\n", len(files))

	b, err := bundle.Deserialize(files)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize bundle: %w", err)
	}

	r.Event("manifest_validated", map[string]any{"version": b.Manifest.Version, "tool": b.Manifest.Tool})
	r.Info("✓ Manifest validated (version %d, tool: %s)\n", b.Manifest.Version, b.Manifest.Tool)

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
		IncludesContents: b.Manifest.IncludesContents,
		ContentsCount:    len(b.Contents),
	}

	if result.Files == nil {
		result.Files = []string{}
	}

	if cfg.OutDir != "" {
		if err := archive.ExtractToDir(cfg.Path, cfg.OutDir); err != nil {
			return nil, fmt.Errorf("failed to extract to directory: %w", err)
		}
		result.ExtractedTo = cfg.OutDir
		r.Event("extracted_to", map[string]any{"path": cfg.OutDir})
		r.Info("✓ Files extracted to: %s\n", cfg.OutDir)
	}

	r.Info("\n════════════════════════════════════════\n")
	r.Info("  Bundle Summary\n")
	r.Info("════════════════════════════════════════\n")
	r.Info("  Project:       %s\n", b.Metadata.ProjectName)
	r.Info("  Branch:        %s\n", b.Metadata.Branch)
	r.Info("  Created:       %s\n", b.Metadata.CreatedAt.Format("2006-01-02 15:04:05 UTC"))
	r.Info("  Generator:     %s\n", b.Metadata.Generator)
	r.Info("  Repository:    %s\n", b.Metadata.RepositoryRoot)
	r.Info("  OS:            %s\n", b.Metadata.OS)
	r.Info("──────────────────────────────────────────\n")
	r.Info("  HEAD:          %s\n", b.Git.HeadCommit)
	r.Info("  Dirty:         %t\n", b.Git.Dirty)
	if b.Git.RemoteURL != "" {
		r.Info("  Remote:        %s\n", b.Git.RemoteURL)
	}
	if b.Git.CurrentTag != "" {
		r.Info("  Tag:           %s\n", b.Git.CurrentTag)
	}
	r.Info("──────────────────────────────────────────\n")
	r.Info("  Files:         %d\n", len(b.Files))
	r.Info("  Prompts:       %d\n", len(b.Prompts))
	r.Info("  Diff present:  %t\n", b.Diff != "")
	if b.Diff != "" {
		r.Info("  Diff size:     %d bytes\n", len(b.Diff))
	}
	r.Info("════════════════════════════════════════\n")

	if len(b.Files) > 0 {
		r.Info("\nModified Files:\n")
		for _, f := range b.Files {
			r.Info("  %s\n", f)
		}
	}

	if b.Diff != "" && cfg.OutDir != "" {
		r.Info("\nTo apply the uncommitted changes:\n")
		r.Info("  git apply %s/patch.diff\n", cfg.OutDir)
	}

	r.Done(result)
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
