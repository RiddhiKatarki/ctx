package bundle

import (
	"strings"
	"testing"
	"time"

	"github.com/RiddhiKatarki/ctx/internal/schema"
	"github.com/RiddhiKatarki/ctx/pkg/types"
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

	b := Build(snapshot, summ, nil)

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

	b := Build(snapshot, summ, nil)

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

	b := Build(snapshot, summ, nil)
	files, err := b.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	if string(files[schema.FilesFile]) != "[]" {
		t.Errorf("expected empty files list '[]', got %s", string(files[schema.FilesFile]))
	}
}

func TestSerialize_WithContents(t *testing.T) {
	snapshot := makeTestSnapshot()
	summ := makeTestSummary()

	contents := map[string][]byte{
		"main.go":  []byte("package main\n"),
		"auth.go":  []byte("package auth\n"),
		"README.md": []byte("# Test\n"),
	}

	b := Build(snapshot, summ, contents)
	files, err := b.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	// Manifest must say includes_contents=true
	if !b.Manifest.IncludesContents {
		t.Error("expected Manifest.IncludesContents=true when contents added")
	}

	if !strings.Contains(string(files[schema.ManifestFile]), `"includes_contents": true`) {
		t.Errorf("manifest.json missing includes_contents field, got: %s", string(files[schema.ManifestFile]))
	}

	// Each entry embedded under contents/<path>
	for rel, data := range contents {
		key := schema.ContentsPrefix + rel
		got, ok := files[key]
		if !ok {
			t.Errorf("missing contents entry: %s", key)
			continue
		}
		if string(got) != string(data) {
			t.Errorf("content mismatch for %s: got %q want %q", key, got, data)
		}
	}

	if _, ok := files["contents/../escape"]; ok {
		t.Error("path traversal in contents should be sanitised")
	}
}

func TestSerialize_ContentsWithoutPathTraversal(t *testing.T) {
	snapshot := makeTestSnapshot()
	summ := makeTestSummary()

	contents := map[string][]byte{
		"../escape.txt": []byte("malicious"),
	}

	b := Build(snapshot, summ, contents)
	files, err := b.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	// fp.Clean(sep + "../escape.txt") = "/escape.txt" — strip leading
	// slash so the resulting key is "contents/escape.txt", not
	// "contents/..".
	if _, ok := files["contents/../escape.txt"]; ok {
		t.Error("path traversal was not normalised")
	}
	if _, ok := files["contents/escape.txt"]; !ok {
		t.Error("expected normalised key contents/escape.txt")
	}
}

func TestDeserialize_ContentsRoundTrip(t *testing.T) {
	snapshot := makeTestSnapshot()
	summ := makeTestSummary()

	contents := map[string][]byte{
		"main.go": []byte("package main\nfunc main() {}\n"),
	}

	b := Build(snapshot, summ, contents)
	files, err := b.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	restored, err := Deserialize(files)
	if err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}

	if !restored.Manifest.IncludesContents {
		t.Error("expected restored manifest to include contents flag")
	}

	if len(restored.Contents) != 1 {
		t.Fatalf("expected 1 content entry, got %d", len(restored.Contents))
	}
	if string(restored.Contents["main.go"]) != string(contents["main.go"]) {
		t.Errorf("content mismatch")
	}
}

func TestDeserialize_NoContents(t *testing.T) {
	snapshot := makeTestSnapshot()
	summ := makeTestSummary()

	b := Build(snapshot, summ, nil)
	files, err := b.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	restored, err := Deserialize(files)
	if err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}

	if restored.Manifest.IncludesContents {
		t.Error("manifest should not include contents when empty")
	}
	if len(restored.Contents) != 0 {
		t.Errorf("expected no contents entries, got %d", len(restored.Contents))
	}
}
