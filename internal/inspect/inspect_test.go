package inspect

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/RiddhiKatarki/ctx/internal/schema"
)

func createInspectTestBundle(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "inspect_test.ctx")

	files := map[string][]byte{
		schema.ManifestFile: []byte(`{"version":1,"created_at":"2024-01-01T00:00:00Z","tool":"ctx"}`),
		schema.MetadataFile: []byte(`{"project_name":"inspect-test","branch":"develop","created_at":"2024-01-01T00:00:00Z","generator":"ctx","repository_root":"/tmp/inspect","os":"linux/amd64"}`),
		schema.GitFile:      []byte(`{"current_branch":"develop","head_commit":"def456","dirty":false}`),
		schema.SummaryFile:  []byte("## Current Objective\n\nImplement feature X\n\n## Known Bugs\n\n- Memory leak\n"),
		schema.PromptsFile:  []byte(`[]`),
		schema.FilesFile:    []byte(`["file1.go","file2.go","file3.go"]`),
		schema.PatchFile:    []byte(""),
	}

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	zw := zip.NewWriter(f)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("failed to create zip entry %s: %v", name, err)
		}
		if _, err := w.Write(content); err != nil {
			t.Fatalf("failed to write zip entry %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("failed to close zip: %v", err)
	}

	return path
}

func TestRun_JSONOutput(t *testing.T) {
	dir := t.TempDir()
	path := createInspectTestBundle(t, dir)

	result, err := Run(Config{Path: path, JSONOutput: true})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil Result")
	}

	if result.Path != path {
		t.Errorf("expected path %s, got %s", path, result.Path)
	}
	if result.FileCount != 3 {
		t.Errorf("expected 3 files, got %d", result.FileCount)
	}
	if len(result.Files) != 3 {
		t.Errorf("expected 3 files in slice, got %d", len(result.Files))
	}
	if !result.Valid {
		t.Error("expected Valid to be true")
	}

	if result.Metadata["project_name"] != "inspect-test" {
		t.Errorf("expected project_name inspect-test, got %v", result.Metadata["project_name"])
	}
	if result.Metadata["branch"] != "develop" {
		t.Errorf("expected branch develop, got %v", result.Metadata["branch"])
	}

	if v, ok := result.Manifest["version"].(int); !ok || v != 1 {
		t.Errorf("expected manifest version 1 (int), got %v (%T)", result.Manifest["version"], result.Manifest["version"])
	}
	if result.Manifest["tool"] != "ctx" {
		t.Errorf("expected tool ctx, got %v", result.Manifest["tool"])
	}

	if result.SummarySections["current_objective"] != "Implement feature X" {
		t.Errorf("expected objective 'Implement feature X', got %q", result.SummarySections["current_objective"])
	}
	if result.SummarySections["known_bugs"] != "- Memory leak" {
		t.Errorf("expected bugs '- Memory leak', got %q", result.SummarySections["known_bugs"])
	}
}

func TestResult_JSONMarshaling(t *testing.T) {
	r := &Result{
		Path:      "test.ctx",
		FileCount: 5,
		Valid:     true,
		Manifest:  map[string]any{"version": 1, "tool": "ctx"},
		Metadata:  map[string]any{"project_name": "p", "branch": "main"},
		Files:     []string{"a.go", "b.go"},
		SummarySections: map[string]string{
			"current_objective": "test",
		},
	}

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	for _, field := range []string{"path", "manifest", "metadata", "files", "file_count", "summary_sections", "valid"} {
		if _, ok := parsed[field]; !ok {
			t.Errorf("expected field %q in JSON output", field)
		}
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

func TestRun_NotAGitPath(t *testing.T) {
	_, err := Run(Config{Path: "/nonexistent/path", JSONOutput: true})
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestIndentLines(t *testing.T) {
	tests := []struct {
		input, indent, expected string
	}{
		{"hello", "  ", "  hello"},
		{"hello\nworld", "  ", "  hello\n  world"},
		{"", "  ", ""},
		{"\n\n", "  ", ""},
	}

	for _, tt := range tests {
		got := indentLines(tt.input, tt.indent)
		if got != tt.expected {
			t.Errorf("indentLines(%q, %q) = %q, expected %q", tt.input, tt.indent, got, tt.expected)
		}
	}
}

func TestHuman_SkippedInJSONMode(t *testing.T) {
	dir := t.TempDir()
	path := createInspectTestBundle(t, dir)

	// Run in JSON mode — should not produce human prints (verified by no panic)
	_, err := Run(Config{Path: path, JSONOutput: true})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
}
