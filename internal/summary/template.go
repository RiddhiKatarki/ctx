package summary

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/context-handoff/ctx/pkg/types"
)

// TemplateProvider generates a structured summary from the snapshot
// using Go text/template. No LLM call is made — this is fully local.
type TemplateProvider struct{}

func NewTemplateProvider() *TemplateProvider {
	return &TemplateProvider{}
}

const summaryTemplate = `## Current Objective

Continue work on the {{.Metadata.ProjectName}} project on branch {{.Metadata.Branch}}. The repository is currently {{if .Git.Dirty}}in a dirty state with uncommitted changes{{else}}clean{{end}} at commit {{.Git.HeadCommit}}.

## Completed Work

{{if .Git.Dirty}}There are uncommitted changes captured in patch.diff. The following files have been modified:{{range .Files}}
  - {{.}}{{end}}{{else}}No uncommitted changes detected. All work appears to be committed.{{end}}

## Remaining Tasks

Review the uncommitted changes in patch.diff and continue implementation. Refer to the modified files list and prompt history for context on what was being worked on.

## Known Bugs

{{if .Diff}}Review the diff for potential issues. The repository is in a dirty state — verify that changes compile and tests pass before continuing.{{else}}No specific bugs documented in this snapshot. Check the prompt history for details.{{end}}

## Architecture Decisions

Project: {{.Metadata.ProjectName}}
Branch: {{.Metadata.Branch}}
HEAD: {{.Git.HeadCommit}}{{if .Git.RemoteURL}}
Remote: {{.Git.RemoteURL}}{{end}}{{if .Git.CurrentTag}}
Tag: {{.Git.CurrentTag}}{{end}}
Operating System: {{.Metadata.OS}}

## Files To Read First

{{if .Files}}The following files have been modified and should be reviewed first:{{range .Files}}
  {{.}}{{end}}{{else}}No modified files detected.{{end}}

## Previous Failed Approaches

No failed approaches were documented in this snapshot. Check the prompt history for context on approaches that were tried.

## Suggested Next Prompt

"Review the current state of the {{.Metadata.ProjectName}} project on branch {{.Metadata.Branch}}. Apply the patch.diff if needed, examine the modified files, and continue implementation based on the completed work and remaining tasks above."

## Estimated Reading Time

~{{.ReadingTime}} minutes
`

func (p *TemplateProvider) Summarize(snapshot types.Snapshot) (*types.Summary, error) {
	readingTime := estimateReadingTime(snapshot)

	tmplData := struct {
		Metadata    types.Metadata
		Git         types.GitMetadata
		Files       []string
		Diff        string
		ReadingTime int
	}{
		Metadata:    snapshot.Metadata,
		Git:         snapshot.Git,
		Files:       snapshot.Files,
		Diff:        snapshot.Diff,
		ReadingTime: readingTime,
	}

	tmpl, err := template.New("summary").Parse(summaryTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse summary template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, tmplData); err != nil {
		return nil, fmt.Errorf("failed to execute summary template: %w", err)
	}

	rendered := buf.String()

	return &types.Summary{
		CurrentObjective:         extractSection(rendered, "Current Objective"),
		CompletedWork:            extractSection(rendered, "Completed Work"),
		RemainingTasks:           extractSection(rendered, "Remaining Tasks"),
		KnownBugs:                extractSection(rendered, "Known Bugs"),
		ArchitectureDecisions:    extractSection(rendered, "Architecture Decisions"),
		FilesToReadFirst:         extractSection(rendered, "Files To Read First"),
		PreviousFailedApproaches: extractSection(rendered, "Previous Failed Approaches"),
		SuggestedNextPrompt:      extractSection(rendered, "Suggested Next Prompt"),
		EstimatedReadingTime:     fmt.Sprintf("~%d minutes", readingTime),
	}, nil
}

func estimateReadingTime(snapshot types.Snapshot) int {
	totalChars := len(snapshot.Diff)
	for _, f := range snapshot.Files {
		totalChars += len(f)
	}
	for _, p := range snapshot.Prompts {
		totalChars += len(p.Content)
	}
	minutes := totalChars / 1500
	if minutes < 1 {
		minutes = 1
	}
	if minutes > 30 {
		minutes = 30
	}
	return minutes
}

func extractSection(content, heading string) string {
	lines := strings.Split(content, "\n")
	var result []string
	capturing := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			currentHeading := strings.TrimPrefix(trimmed, "## ")
			if currentHeading == heading {
				capturing = true
				continue
			} else if capturing {
				break
			}
		} else if capturing {
			result = append(result, line)
		}
	}

	return strings.TrimSpace(strings.Join(result, "\n"))
}

// RenderMarkdown converts a Summary struct back into the summary.md markdown format.
func RenderMarkdown(s *types.Summary) string {
	var buf bytes.Buffer
	buf.WriteString("## Current Objective\n\n")
	buf.WriteString(s.CurrentObjective)
	buf.WriteString("\n\n## Completed Work\n\n")
	buf.WriteString(s.CompletedWork)
	buf.WriteString("\n\n## Remaining Tasks\n\n")
	buf.WriteString(s.RemainingTasks)
	buf.WriteString("\n\n## Known Bugs\n\n")
	buf.WriteString(s.KnownBugs)
	buf.WriteString("\n\n## Architecture Decisions\n\n")
	buf.WriteString(s.ArchitectureDecisions)
	buf.WriteString("\n\n## Files To Read First\n\n")
	buf.WriteString(s.FilesToReadFirst)
	buf.WriteString("\n\n## Previous Failed Approaches\n\n")
	buf.WriteString(s.PreviousFailedApproaches)
	buf.WriteString("\n\n## Suggested Next Prompt\n\n")
	buf.WriteString(s.SuggestedNextPrompt)
	buf.WriteString("\n\n## Estimated Reading Time\n\n")
	buf.WriteString(s.EstimatedReadingTime)
	buf.WriteString("\n")
	return buf.String()
}

// ParseMarkdownSummary is a placeholder for parsing LLM-generated markdown
// back into a structured Summary. Used by the OpenAI provider.
func ParseMarkdownSummary(content string) *types.Summary {
	return &types.Summary{
		CurrentObjective:         extractSection(content, "Current Objective"),
		CompletedWork:            extractSection(content, "Completed Work"),
		RemainingTasks:           extractSection(content, "Remaining Tasks"),
		KnownBugs:                extractSection(content, "Known Bugs"),
		ArchitectureDecisions:    extractSection(content, "Architecture Decisions"),
		FilesToReadFirst:         extractSection(content, "Files To Read First"),
		PreviousFailedApproaches: extractSection(content, "Previous Failed Approaches"),
		SuggestedNextPrompt:      extractSection(content, "Suggested Next Prompt"),
		EstimatedReadingTime:     extractSection(content, "Estimated Reading Time"),
	}
}
