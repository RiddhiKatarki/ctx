package bundle

import (
	"testing"
	"time"

	"github.com/context-handoff/ctx/internal/schema"
	"github.com/context-handoff/ctx/pkg/types"
)

func makeTestSnapshot() types.Snapshot {
	return types.Snapshot{
		Metadata: types.Metadata{
			ProjectName: "test-project",
			Branch:      "main",
			CreatedAt:   time.Now().UTC(),
			Generator:   "ctx",
			OS:          "linux/amd64",
		},
		Git: types.GitMetadata{
			CurrentBranch: "main",
			HeadCommit:    "abc123",
			Dirty:         true,
			RemoteURL:     "git@github.com:user/repo.git",
		},
		Prompts: []types.Prompt{
			{Role: "user", Content: "Do something"},
		},
		Files: []string{"main.go", "auth.go"},
		Diff:  "diff --git a/main.go b/main.go\n+hello",
	}
}

func makeTestSummary() *types.Summary {
	return &types.Summary{
		CurrentObjective:     "Test objective",
		CompletedWork:        "Test work",
		RemainingTasks:       "Test tasks",
		EstimatedReadingTime: "~2 minutes",
	}
}

func TestBuild_CreatesValidBundle(t *testing.T) {
	snapshot := makeTestSnapshot()
	summ := makeTestSummary()

	b := Build(snapshot, summ)

	if b.Manifest.Version != schema.BundleVersion {
		t.Errorf("expected version %d, got %d", schema.BundleVersion, b.Manifest.Version)
	}
	if b.Manifest.Tool != schema.ToolName {
		t.Errorf("expected tool %s, got %s", schema.ToolName, b.Manifest.Tool)
	}
	if b.Metadata.ProjectName != "test-project" {
		t.Errorf("expected test-project, got %s", b.Metadata.ProjectName)
	}
	if b.Git.HeadCommit != "abc123" {
		t.Errorf("expected abc123, got %s", b.Git.HeadCommit)
	}
	if len(b.Prompts) != 1 {
		t.Errorf("expected 1 prompt, got %d", len(b.Prompts))
	}
}

func TestSerialize_Deserialize_RoundTrip(t *testing.T) {
	snapshot := makeTestSnapshot()
	summ := makeTestSummary()

	b := Build(snapshot, summ)

	files, err := b.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	for _, required := range schema.RequiredFiles {
		if _, ok := files[required]; !ok {
			t.Errorf("serialized bundle missing required file: %s", required)
		}
	}

	restored, err := Deserialize(files)
	if err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}

	if restored.Metadata.ProjectName != b.Metadata.ProjectName {
		t.Errorf("ProjectName mismatch: %s vs %s", restored.Metadata.ProjectName, b.Metadata.ProjectName)
	}
	if restored.Git.HeadCommit != b.Git.HeadCommit {
		t.Errorf("HeadCommit mismatch: %s vs %s", restored.Git.HeadCommit, b.Git.HeadCommit)
	}
	if restored.Git.Dirty != b.Git.Dirty {
		t.Errorf("Dirty mismatch: %v vs %v", restored.Git.Dirty, b.Git.Dirty)
	}
	if len(restored.Prompts) != len(b.Prompts) {
		t.Errorf("Prompts length mismatch: %d vs %d", len(restored.Prompts), len(b.Prompts))
	}
	if len(restored.Files) != len(b.Files) {
		t.Errorf("Files length mismatch: %d vs %d", len(restored.Files), len(b.Files))
	}
	if restored.Diff != b.Diff {
		t.Errorf("Diff mismatch")
	}
	if restored.Summary.CurrentObjective != b.Summary.CurrentObjective {
		t.Errorf("Summary mismatch")
	}
}

func TestDeserialize_MissingManifest(t *testing.T) {
	files := map[string][]byte{
		"metadata.json": []byte(`{"project_name":"test"}`),
	}
	_, err := Deserialize(files)
	if err == nil {
		t.Error("expected error when manifest is missing")
	}
}

func TestDeserialize_InvalidManifestVersion(t *testing.T) {
	files := map[string][]byte{
		"manifest.json": []byte(`{"version":999,"created_at":"2024-01-01T00:00:00Z","tool":"ctx"}`),
	}
	_, err := Deserialize(files)
	if err == nil {
		t.Error("expected error for invalid manifest version")
	}
}

func TestSerialize_EmptyFiles(t *testing.T) {
	snapshot := makeTestSnapshot()
	snapshot.Files = nil
	summ := makeTestSummary()

	b := Build(snapshot, summ)
	files, err := b.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	if string(files[schema.FilesFile]) != "[]" {
		t.Errorf("expected empty files list '[]', got %s", string(files[schema.FilesFile]))
	}
}
