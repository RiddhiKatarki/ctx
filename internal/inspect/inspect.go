package inspect

import (
	"fmt"
	"strings"

	"github.com/RiddhiKatarki/ctx/internal/archive"
	"github.com/RiddhiKatarki/ctx/internal/reporter"
	"github.com/RiddhiKatarki/ctx/internal/summary"
)

// Config holds options for the inspect operation.
type Config struct {
	Path string

	// Reporter controls output for stream/json modes.
	Reporter reporter.Reporter

	// JSONOutput retained for backwards compat — auto creates JSONReporter.
	JSONOutput bool
}

// Result is the structured output of an inspect operation.
// JSON tags enable machine-readable output when emitted with --json.
type Result struct {
	Path            string            `json:"path"`
	Manifest        map[string]any    `json:"manifest"`
	Metadata        map[string]any    `json:"metadata"`
	Files           []string          `json:"files"`
	FileCount       int               `json:"file_count"`
	SummarySections map[string]string `json:"summary_sections"`
	Valid           bool              `json:"valid"`
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

// Run reads a .ctx bundle without importing and returns a structured Result.
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

	peek, err := archive.Peek(cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect archive: %w", err)
	}
	r.Event("archive_read", map[string]any{"path": cfg.Path, "version": peek.Manifest.Version})

	projectName := "Unknown"
	if peek.Metadata != nil {
		projectName = peek.Metadata.ProjectName
	}

	branch := "Unknown"
	if peek.Metadata != nil {
		branch = peek.Metadata.Branch
	}

	summaryText := string(peek.Summary)
	parsed := summary.ParseMarkdownSummary(summaryText)

	sections := map[string]string{
		"current_objective":          parsed.CurrentObjective,
		"completed_work":             parsed.CompletedWork,
		"remaining_tasks":            parsed.RemainingTasks,
		"known_bugs":                 parsed.KnownBugs,
		"architecture_decisions":     parsed.ArchitectureDecisions,
		"files_to_read_first":        parsed.FilesToReadFirst,
		"previous_failed_approaches": parsed.PreviousFailedApproaches,
		"suggested_next_prompt":      parsed.SuggestedNextPrompt,
		"estimated_reading_time":     parsed.EstimatedReadingTime,
	}

	result := &Result{
		Path: cfg.Path,
		Manifest: map[string]any{
			"version":    0,
			"created_at": "",
			"tool":       "",
		},
		Metadata: map[string]any{
			"project_name":    projectName,
			"branch":          branch,
			"created_at":      "",
			"generator":       "",
			"repository_root": "",
			"os":              "",
		},
		Files:           peek.Files,
		FileCount:       len(peek.Files),
		SummarySections: sections,
		Valid:           true,
	}

	if peek.Manifest != nil {
		result.Manifest["version"] = peek.Manifest.Version
		result.Manifest["created_at"] = peek.Manifest.CreatedAt
		result.Manifest["tool"] = peek.Manifest.Tool
	}
	if peek.Metadata != nil {
		result.Metadata["created_at"] = peek.Metadata.CreatedAt
		result.Metadata["generator"] = peek.Metadata.Generator
		result.Metadata["repository_root"] = peek.Metadata.RepositoryRoot
		result.Metadata["os"] = peek.Metadata.OS
	}

	if result.Files == nil {
		result.Files = []string{}
	}

	r.Info("Project: %s\n\n", projectName)

	r.Info("Branch:\n")
	r.Info("  %s\n\n", branch)

	r.Info("Current Goal:\n")
	if parsed.CurrentObjective != "" {
		r.Info("  %s\n", indentLines(parsed.CurrentObjective, "  "))
	} else {
		r.Info("  (not specified)\n")
	}
	r.Info("\n")

	r.Info("Modified Files:\n")
	r.Info("  %d\n\n", len(peek.Files))

	r.Info("Known Bug:\n")
	if parsed.KnownBugs != "" {
		r.Info("  %s\n", indentLines(parsed.KnownBugs, "  "))
	} else {
		r.Info("  (none documented)\n")
	}
	r.Info("\n")

	r.Info("Completed Work:\n")
	if parsed.CompletedWork != "" {
		r.Info("  %s\n", indentLines(parsed.CompletedWork, "  "))
	} else {
		r.Info("  (not specified)\n")
	}
	r.Info("\n")

	r.Info("Remaining Tasks:\n")
	if parsed.RemainingTasks != "" {
		r.Info("  %s\n", indentLines(parsed.RemainingTasks, "  "))
	} else {
		r.Info("  (not specified)\n")
	}
	r.Info("\n")

	r.Info("Architecture Decisions:\n")
	if parsed.ArchitectureDecisions != "" {
		r.Info("  %s\n", indentLines(parsed.ArchitectureDecisions, "  "))
	} else {
		r.Info("  (not specified)\n")
	}
	r.Info("\n")

	r.Info("Files To Read First:\n")
	if parsed.FilesToReadFirst != "" {
		r.Info("  %s\n", indentLines(parsed.FilesToReadFirst, "  "))
	} else {
		r.Info("  (not specified)\n")
	}
	r.Info("\n")

	r.Info("Previous Failed Approaches:\n")
	if parsed.PreviousFailedApproaches != "" {
		r.Info("  %s\n", indentLines(parsed.PreviousFailedApproaches, "  "))
	} else {
		r.Info("  (none documented)\n")
	}
	r.Info("\n")

	r.Info("Suggested Next Prompt:\n")
	if parsed.SuggestedNextPrompt != "" {
		r.Info("  %s\n", indentLines(parsed.SuggestedNextPrompt, "  "))
	} else {
		r.Info("  (not specified)\n")
	}
	r.Info("\n")

	r.Info("Estimated Reading Time:\n")
	if parsed.EstimatedReadingTime != "" {
		r.Info("  %s\n", parsed.EstimatedReadingTime)
	} else {
		r.Info("  (not specified)\n")
	}

	r.Done(result)
	return result, nil
}

func indentLines(text, indent string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) != "" {
			lines[i] = indent + line
		}
	}
	return strings.Join(lines, "\n")
}
