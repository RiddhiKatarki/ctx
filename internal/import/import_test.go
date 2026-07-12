package importctx

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/RiddhiKatarki/ctx/internal/schema"
)

func createTestBundle(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "test.ctx")

	files := map[string][]byte{
		schema.ManifestFile: []byte(`{"version":1,"created_at":"2024-01-01T00:00:00Z","tool":"ctx"}`),
		schema.MetadataFile: []byte(`{"project_name":"test","branch":"main","created_at":"2024-01-01T00:00:00Z","generator":"ctx","repository_root":"/tmp/test","os":"linux/amd64"}`),
		schema.GitFile:      []byte(`{"current_branch":"main","head_commit":"abc123","dirty":true,"remote_url":"git@github.com:user/repo.git"}`),
		schema.SummaryFile:  []byte("## Current Objective\n\nTest objective\n"),
		schema.PromptsFile:  []byte(`[{"role":"user","content":"test prompt"}]`),
		schema.FilesFile:    []byte(`["main.go","auth.go"]`),
		schema.PatchFile:    []byte("diff --git a/main.go b/main.go\n+hello"),
	}

	zw, err := zipWriter(path)
	if err != nil {
		t.Fatalf("failed to open zip: %v", err)
	}
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("failed to create zip entry: %v", err)
		}
		if _, err := w.Write(content); err != nil {
			t.Fatalf("failed to write zip entry: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("failed to close zip: %v", err)
	}
	return path
}

func zipWriter(path string) (*zip.Writer, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	return zip.NewWriter(f), nil
}

func TestRun_JSONOutput_FlagSet(t *testing.T) {
	dir := t.TempDir()
	path := createTestBundle(t, dir)

	result, err := Run(Config{Path: path, JSONOutput: true})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Verify the struct fields
	if result.Path != path {
		t.Errorf("expected path %s, got %s", path, result.Path)
	}
	if result.ProjectName != "test" {
		t.Errorf("expected project test, got %s", result.ProjectName)
	}
	if result.Branch != "main" {
		t.Errorf("expected branch main, got %s", result.Branch)
	}
	if result.FileCount != 2 {
		t.Errorf("expected 2 files, got %d", result.FileCount)
	}
	if result.PromptCount != 1 {
		t.Errorf("expected 1 prompt, got %d", result.PromptCount)
	}
	if !result.HasDiff {
		t.Error("expected HasDiff to be true")
	}
	if result.Commit != "abc123" {
		t.Errorf("expected commit abc123, got %s", result.Commit)
	}
	if !result.Valid {
		t.Error("expected Valid to be true")
	}

	// Verify JSON marshaling produces valid JSON
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty JSON output")
	}

	// Verify the JSON contains expected fields
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}
	if parsed["path"] != path {
		t.Error("expected path field in JSON")
	}
	if parsed["project_name"] != "test" {
		t.Error("expected project_name field in JSON")
	}
}

func TestRun_JSONOutput_FlagNotSet(t *testing.T) {
	dir := t.TempDir()
	path := createTestBundle(t, dir)

	result, err := Run(Config{Path: path, JSONOutput: false})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result even without --json")
	}
	if result.ProjectName != "test" {
		t.Errorf("expected project test, got %s", result.ProjectName)
	}
}

func TestRun_EmptyPath(t *testing.T) {
	_, err := Run(Config{Path: "", JSONOutput: true})
	if err == nil {
		t.Error("expected error for empty path")
	}
}

func TestRun_NonZipFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notzip.ctx")
	if err := os.WriteFile(path, []byte("not a zip"), 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	_, err := Run(Config{Path: path, JSONOutput: true})
	if err == nil {
		t.Error("expected error for non-ZIP file")
	}
}

func TestResult_JSONTags(t *testing.T) {
	r := &Result{
		Path:            "test.ctx",
		ManifestVersion: 1,
		Tool:            "ctx",
		ProjectName:     "myproject",
		Branch:          "main",
		FileCount:       4,
		PromptCount:     2,
		HasDiff:         true,
		Valid:           true,
	}

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	expectedFields := []string{
		"path", "manifest_version", "tool", "project_name", "branch",
		"file_count", "prompt_count", "has_diff", "valid",
	}
	for _, field := range expectedFields {
		if _, ok := parsed[field]; !ok {
			t.Errorf("expected field %q in JSON output", field)
		}
	}
}

func TestQuickSummary(t *testing.T) {
	r := &Result{ProjectName: "p", Branch: "main", FileCount: 3, PromptCount: 1}
	s := QuickSummary(r)
	if s == "" {
		t.Error("expected non-empty summary")
	}
	if !contains(s, "p") || !contains(s, "main") {
		t.Errorf("summary missing data: %s", s)
	}
}

func contains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
