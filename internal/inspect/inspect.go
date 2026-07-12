package inspect

import (
	"fmt"
	"strings"

	"github.com/RiddhiKatarki/ctx/internal/archive"
	"github.com/RiddhiKatarki/ctx/internal/summary"
)

// Config holds options for the inspect operation.
type Config struct {
	Path string

	// JSONOutput, when true, suppresses human-readable prints
	// and the caller is expected to encode the Result as JSON.
	JSONOutput bool
}

// Result is the structured output of an inspect operation.
// JSON tags enable machine-readable output when emitted with --json.
type Result struct {
	Path           string            `json:"path"`
	Manifest       map[string]any    `json:"manifest"`
	Metadata       map[string]any    `json:"metadata"`
	Files          []string          `json:"files"`
	FileCount      int               `json:"file_count"`
	Summary        string            `json:"summary"`
	SummarySections map[string]string `json:"summary_sections"`
	Valid          bool              `json:"valid"`
}

func (cfg Config) human(format string, a ...any) {
	if cfg.JSONOutput {
		return
	}
	fmt.Printf(format, a...)
}

// Run reads a .ctx bundle without importing and returns a structured Result.
func Run(cfg Config) (*Result, error) {
	if cfg.Path == "" {
		return nil, fmt.Errorf("no bundle path specified")
	}

	if !archive.IsZipFile(cfg.Path) {
		return nil, fmt.Errorf("%s is not a valid .ctx archive (not a ZIP file)", cfg.Path)
	}

	peek, err := archive.Peek(cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect archive: %w", err)
	}

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
		"current_objective":         parsed.CurrentObjective,
		"completed_work":            parsed.CompletedWork,
		"remaining_tasks":           parsed.RemainingTasks,
		"known_bugs":                parsed.KnownBugs,
		"architecture_decisions":    parsed.ArchitectureDecisions,
		"files_to_read_first":       parsed.FilesToReadFirst,
		"previous_failed_approaches": parsed.PreviousFailedApproaches,
		"suggested_next_prompt":     parsed.SuggestedNextPrompt,
		"estimated_reading_time":    parsed.EstimatedReadingTime,
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
		Summary:         summaryText,
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

	cfg.human("Project: %s\n\n", projectName)

	cfg.human("Branch:\n")
	cfg.human("  %s\n\n", branch)

	cfg.human("Current Goal:\n")
	if parsed.CurrentObjective != "" {
		cfg.human("  %s\n", indentLines(parsed.CurrentObjective, "  "))
	} else {
		cfg.human("  (not specified)\n")
	}
	cfg.human("\n")

	cfg.human("Modified Files:\n")
	cfg.human("  %d\n\n", len(peek.Files))

	cfg.human("Known Bug:\n")
	if parsed.KnownBugs != "" {
		cfg.human("  %s\n", indentLines(parsed.KnownBugs, "  "))
	} else {
		cfg.human("  (none documented)\n")
	}
	cfg.human("\n")

	cfg.human("Completed Work:\n")
	if parsed.CompletedWork != "" {
		cfg.human("  %s\n", indentLines(parsed.CompletedWork, "  "))
	} else {
		cfg.human("  (not specified)\n")
	}
	cfg.human("\n")

	cfg.human("Remaining Tasks:\n")
	if parsed.RemainingTasks != "" {
		cfg.human("  %s\n", indentLines(parsed.RemainingTasks, "  "))
	} else {
		cfg.human("  (not specified)\n")
	}
	cfg.human("\n")

	cfg.human("Architecture Decisions:\n")
	if parsed.ArchitectureDecisions != "" {
		cfg.human("  %s\n", indentLines(parsed.ArchitectureDecisions, "  "))
	} else {
		cfg.human("  (not specified)\n")
	}
	cfg.human("\n")

	cfg.human("Files To Read First:\n")
	if parsed.FilesToReadFirst != "" {
		cfg.human("  %s\n", indentLines(parsed.FilesToReadFirst, "  "))
	} else {
		cfg.human("  (not specified)\n")
	}
	cfg.human("\n")

	cfg.human("Previous Failed Approaches:\n")
	if parsed.PreviousFailedApproaches != "" {
		cfg.human("  %s\n", indentLines(parsed.PreviousFailedApproaches, "  "))
	} else {
		cfg.human("  (none documented)\n")
	}
	cfg.human("\n")

	cfg.human("Suggested Next Prompt:\n")
	if parsed.SuggestedNextPrompt != "" {
		cfg.human("  %s\n", indentLines(parsed.SuggestedNextPrompt, "  "))
	} else {
		cfg.human("  (not specified)\n")
	}
	cfg.human("\n")

	cfg.human("Estimated Reading Time:\n")
	if parsed.EstimatedReadingTime != "" {
		cfg.human("  %s\n", parsed.EstimatedReadingTime)
	} else {
		cfg.human("  (not specified)\n")
	}

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
