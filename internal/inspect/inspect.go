package inspect

import (
	"fmt"
	"strings"

	"github.com/context-handoff/ctx/internal/archive"
	"github.com/context-handoff/ctx/internal/summary"
)

// Config holds options for the inspect operation.
type Config struct {
	Path string
}

// Run reads a .ctx bundle without importing and displays summary information.
func Run(cfg Config) error {
	if cfg.Path == "" {
		return fmt.Errorf("no bundle path specified")
	}

	if !archive.IsZipFile(cfg.Path) {
		return fmt.Errorf("%s is not a valid .ctx archive (not a ZIP file)", cfg.Path)
	}

	peek, err := archive.Peek(cfg.Path)
	if err != nil {
		return fmt.Errorf("failed to inspect archive: %w", err)
	}

	projectName := "Unknown"
	if peek.Metadata != nil {
		projectName = peek.Metadata.ProjectName
	}

	branch := "Unknown"
	if peek.Metadata != nil {
		branch = peek.Metadata.Branch
	}

	parsed := summary.ParseMarkdownSummary(string(peek.Summary))

	fileCount := len(peek.Files)

	fmt.Printf("Project: %s\n", projectName)
	fmt.Println()
	fmt.Println("Branch:")
	fmt.Printf("  %s\n", branch)
	fmt.Println()
	fmt.Println("Current Goal:")
	if parsed.CurrentObjective != "" {
		fmt.Printf("  %s\n", indentLines(parsed.CurrentObjective, "  "))
	} else {
		fmt.Println("  (not specified)")
	}
	fmt.Println()
	fmt.Printf("Modified Files:\n")
	fmt.Printf("  %d\n", fileCount)
	fmt.Println()
	fmt.Println("Known Bug:")
	if parsed.KnownBugs != "" {
		fmt.Printf("  %s\n", indentLines(parsed.KnownBugs, "  "))
	} else {
		fmt.Println("  (none documented)")
	}
	fmt.Println()
	fmt.Println("Completed Work:")
	if parsed.CompletedWork != "" {
		fmt.Printf("  %s\n", indentLines(parsed.CompletedWork, "  "))
	} else {
		fmt.Println("  (not specified)")
	}
	fmt.Println()
	fmt.Println("Remaining Tasks:")
	if parsed.RemainingTasks != "" {
		fmt.Printf("  %s\n", indentLines(parsed.RemainingTasks, "  "))
	} else {
		fmt.Println("  (not specified)")
	}
	fmt.Println()
	fmt.Println("Architecture Decisions:")
	if parsed.ArchitectureDecisions != "" {
		fmt.Printf("  %s\n", indentLines(parsed.ArchitectureDecisions, "  "))
	} else {
		fmt.Println("  (not specified)")
	}
	fmt.Println()
	fmt.Println("Files To Read First:")
	if parsed.FilesToReadFirst != "" {
		fmt.Printf("  %s\n", indentLines(parsed.FilesToReadFirst, "  "))
	} else {
		fmt.Println("  (not specified)")
	}
	fmt.Println()
	fmt.Println("Previous Failed Approaches:")
	if parsed.PreviousFailedApproaches != "" {
		fmt.Printf("  %s\n", indentLines(parsed.PreviousFailedApproaches, "  "))
	} else {
		fmt.Println("  (none documented)")
	}
	fmt.Println()
	fmt.Println("Suggested Next Prompt:")
	if parsed.SuggestedNextPrompt != "" {
		fmt.Printf("  %s\n", indentLines(parsed.SuggestedNextPrompt, "  "))
	} else {
		fmt.Println("  (not specified)")
	}
	fmt.Println()
	fmt.Println("Estimated Reading Time:")
	if parsed.EstimatedReadingTime != "" {
		fmt.Printf("  %s\n", parsed.EstimatedReadingTime)
	} else {
		fmt.Println("  (not specified)")
	}

	return nil
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
