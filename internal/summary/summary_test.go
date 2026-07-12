package summary

import (
	"testing"

	"github.com/context-handoff/ctx/pkg/types"
)

func makeTestSnapshot() types.Snapshot {
	return types.Snapshot{
		Metadata: types.Metadata{
			ProjectName: "test-project",
			Branch:      "feature/test",
			OS:          "linux/amd64",
		},
		Git: types.GitMetadata{
			CurrentBranch: "feature/test",
			HeadCommit:    "abc123def456789",
			Dirty:         true,
			RemoteURL:     "git@github.com:user/repo.git",
			CurrentTag:    "v1.0.0",
		},
		Prompts: []types.Prompt{
			{Role: "user", Content: "Implement retry logic."},
			{Role: "assistant", Content: "I'll add a retry wrapper."},
		},
		Files: []string{"main.go", "auth.go", "README.md"},
		Diff:  "diff --git a/main.go b/main.go\n+func newFunc() {}",
	}
}

func TestTemplateProvider_Summarize(t *testing.T) {
	p := NewTemplateProvider()
	snapshot := makeTestSnapshot()

	summ, err := p.Summarize(snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if summ.CurrentObjective == "" {
		t.Error("expected non-empty CurrentObjective")
	}
	if summ.CompletedWork == "" {
		t.Error("expected non-empty CompletedWork")
	}
	if summ.RemainingTasks == "" {
		t.Error("expected non-empty RemainingTasks")
	}
	if summ.KnownBugs == "" {
		t.Error("expected non-empty KnownBugs")
	}
	if summ.ArchitectureDecisions == "" {
		t.Error("expected non-empty ArchitectureDecisions")
	}
	if summ.FilesToReadFirst == "" {
		t.Error("expected non-empty FilesToReadFirst")
	}
	if summ.SuggestedNextPrompt == "" {
		t.Error("expected non-empty SuggestedNextPrompt")
	}
	if summ.EstimatedReadingTime == "" {
		t.Error("expected non-empty EstimatedReadingTime")
	}
}

func TestTemplateProvider_ContainsProjectName(t *testing.T) {
	p := NewTemplateProvider()
	snapshot := makeTestSnapshot()

	summ, err := p.Summarize(snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !contains(summ.CurrentObjective, "test-project") {
		t.Error("expected CurrentObjective to contain project name")
	}
	if !contains(summ.CurrentObjective, "feature/test") {
		t.Error("expected CurrentObjective to contain branch name")
	}
}

func TestTemplateProvider_CleanRepo(t *testing.T) {
	p := NewTemplateProvider()
	snapshot := makeTestSnapshot()
	snapshot.Git.Dirty = false
	snapshot.Diff = ""
	snapshot.Files = nil

	summ, err := p.Summarize(snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !contains(summ.CompletedWork, "No uncommitted changes") {
		t.Error("expected CompletedWork to mention clean state")
	}
}

func TestTemplateProvider_ContainsFiles(t *testing.T) {
	p := NewTemplateProvider()
	snapshot := makeTestSnapshot()

	summ, err := p.Summarize(snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !contains(summ.FilesToReadFirst, "main.go") {
		t.Error("expected FilesToReadFirst to contain main.go")
	}
	if !contains(summ.FilesToReadFirst, "auth.go") {
		t.Error("expected FilesToReadFirst to contain auth.go")
	}
}

func TestEstimateReadingTime(t *testing.T) {
	snapshot := makeTestSnapshot()
	minutes := estimateReadingTime(snapshot)
	if minutes < 1 {
		t.Errorf("expected at least 1 minute, got %d", minutes)
	}
	if minutes > 30 {
		t.Errorf("expected at most 30 minutes, got %d", minutes)
	}
}

func TestRenderMarkdown_AllSections(t *testing.T) {
	s := &types.Summary{
		CurrentObjective:         "Test objective",
		CompletedWork:            "Test work",
		RemainingTasks:           "Test tasks",
		KnownBugs:                "Test bugs",
		ArchitectureDecisions:    "Test decisions",
		FilesToReadFirst:         "Test files",
		PreviousFailedApproaches: "Test failures",
		SuggestedNextPrompt:      "Test prompt",
		EstimatedReadingTime:     "~5 minutes",
	}

	md := RenderMarkdown(s)

	sections := []string{
		"## Current Objective",
		"## Completed Work",
		"## Remaining Tasks",
		"## Known Bugs",
		"## Architecture Decisions",
		"## Files To Read First",
		"## Previous Failed Approaches",
		"## Suggested Next Prompt",
		"## Estimated Reading Time",
	}
	for _, s := range sections {
		if !contains(md, s) {
			t.Errorf("expected markdown to contain %q", s)
		}
	}
}

func TestParseMarkdownSummary_RoundTrip(t *testing.T) {
	original := &types.Summary{
		CurrentObjective:         "Objective here",
		CompletedWork:            "Work done here",
		RemainingTasks:           "Tasks left here",
		KnownBugs:                "Bugs found here",
		ArchitectureDecisions:    "Decisions made here",
		FilesToReadFirst:         "Files to read here",
		PreviousFailedApproaches: "Failed approaches here",
		SuggestedNextPrompt:      "Next prompt here",
		EstimatedReadingTime:     "~3 minutes",
	}

	md := RenderMarkdown(original)
	parsed := ParseMarkdownSummary(md)

	if parsed.CurrentObjective != original.CurrentObjective {
		t.Errorf("CurrentObjective: expected %q, got %q", original.CurrentObjective, parsed.CurrentObjective)
	}
	if parsed.CompletedWork != original.CompletedWork {
		t.Errorf("CompletedWork: expected %q, got %q", original.CompletedWork, parsed.CompletedWork)
	}
	if parsed.EstimatedReadingTime != original.EstimatedReadingTime {
		t.Errorf("EstimatedReadingTime: expected %q, got %q", original.EstimatedReadingTime, parsed.EstimatedReadingTime)
	}
}

func TestNewSummaryProvider_Template(t *testing.T) {
	p, err := NewSummaryProvider(Options{Provider: "template"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := p.(*TemplateProvider); !ok {
		t.Error("expected TemplateProvider")
	}
}

func TestNewSummaryProvider_Default(t *testing.T) {
	p, err := NewSummaryProvider(Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := p.(*TemplateProvider); !ok {
		t.Error("expected TemplateProvider for empty provider name")
	}
}

func TestNewSummaryProvider_OpenAI_NoKey(t *testing.T) {
	_, err := NewSummaryProvider(Options{Provider: "openai"})
	if err == nil {
		t.Error("expected error when no API key provided")
	}
}

func TestNewSummaryProvider_OpenAI_WithKey(t *testing.T) {
	p, err := NewSummaryProvider(Options{Provider: "openai", APIKey: "test-key"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := p.(*OpenAIProvider); !ok {
		t.Error("expected OpenAIProvider")
	}
}

func TestNewSummaryProvider_OpenAI_Defaults(t *testing.T) {
	p, err := NewSummaryProvider(Options{Provider: "openai", APIKey: "test-key"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	oai, ok := p.(*OpenAIProvider)
	if !ok {
		t.Fatal("expected OpenAIProvider")
	}
	if oai.baseURL != "https://api.openai.com/v1" {
		t.Errorf("expected default base URL, got %s", oai.baseURL)
	}
	if oai.model != "gpt-4o" {
		t.Errorf("expected default model gpt-4o, got %s", oai.model)
	}
}

func TestNewSummaryProvider_Unknown(t *testing.T) {
	_, err := NewSummaryProvider(Options{Provider: "unknown"})
	if err == nil {
		t.Error("expected error for unknown provider")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
