package archive

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/context-handoff/ctx/internal/schema"
)

func TestEnsureCtxExtension_AlreadyHasExtension(t *testing.T) {
	result := EnsureCtxExtension("project.ctx")
	if result != "project.ctx" {
		t.Errorf("expected project.ctx, got %s", result)
	}
}

func TestEnsureCtxExtension_NeedsExtension(t *testing.T) {
	result := EnsureCtxExtension("project")
	if result != "project.ctx" {
		t.Errorf("expected project.ctx, got %s", result)
	}
}

func TestEnsureCtxExtension_PathWithDir(t *testing.T) {
	result := EnsureCtxExtension("/tmp/output/mybundle")
	if result != "/tmp/output/mybundle.ctx" {
		t.Errorf("expected /tmp/output/mybundle.ctx, got %s", result)
	}
}

func TestCreate_Extract_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.ctx")

	files := map[string][]byte{
		schema.ManifestFile: []byte(`{"version":1,"created_at":"2024-01-01T00:00:00Z","tool":"ctx"}`),
		schema.MetadataFile: []byte(`{"project_name":"test","branch":"main","created_at":"2024-01-01T00:00:00Z","generator":"ctx","repository_root":"/tmp","os":"linux/amd64"}`),
		schema.GitFile:      []byte(`{"current_branch":"main","head_commit":"abc123","dirty":true}`),
		schema.SummaryFile:  []byte("## Current Objective\n\nTest objective\n"),
		schema.PromptsFile:  []byte(`[{"role":"user","content":"test"}]`),
		schema.FilesFile:    []byte(`["main.go"]`),
		schema.PatchFile:    []byte("diff --git a/main.go b/main.go\n+hello"),
	}

	if err := Create(path, files); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if !IsZipFile(path) {
		t.Error("expected created file to be a valid ZIP")
	}

	extracted, err := Extract(path)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	for filename, expected := range files {
		got, ok := extracted[filename]
		if !ok {
			t.Errorf("extracted bundle missing %s", filename)
			continue
		}
		if string(got) != string(expected) {
			t.Errorf("content mismatch for %s", filename)
		}
	}
}

func TestExtract_NonExistentFile(t *testing.T) {
	_, err := Extract("/nonexistent/file.ctx")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestExtract_NotAZip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notzip.ctx")
	if err := os.WriteFile(path, []byte("not a zip file"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := Extract(path)
	if err == nil {
		t.Error("expected error for non-ZIP file")
	}
}

func TestPeek(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "peek.ctx")

	files := map[string][]byte{
		schema.ManifestFile: []byte(`{"version":1,"created_at":"2024-01-01T00:00:00Z","tool":"ctx"}`),
		schema.MetadataFile: []byte(`{"project_name":"peek-test","branch":"develop","created_at":"2024-01-01T00:00:00Z","generator":"ctx","repository_root":"/tmp","os":"linux/amd64"}`),
		schema.GitFile:      []byte(`{"current_branch":"develop","head_commit":"def456","dirty":false}`),
		schema.SummaryFile:  []byte("## Current Objective\n\nPeek test\n"),
		schema.PromptsFile:  []byte(`[]`),
		schema.FilesFile:    []byte(`["file1.go","file2.go"]`),
		schema.PatchFile:    []byte(""),
	}

	if err := Create(path, files); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	result, err := Peek(path)
	if err != nil {
		t.Fatalf("Peek failed: %v", err)
	}

	if result.Manifest == nil {
		t.Fatal("expected non-nil manifest")
	}
	if result.Manifest.Version != 1 {
		t.Errorf("expected version 1, got %d", result.Manifest.Version)
	}
	if result.Metadata == nil {
		t.Fatal("expected non-nil metadata")
	}
	if result.Metadata.ProjectName != "peek-test" {
		t.Errorf("expected peek-test, got %s", result.Metadata.ProjectName)
	}
	if result.Metadata.Branch != "develop" {
		t.Errorf("expected develop, got %s", result.Metadata.Branch)
	}
	if len(result.Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(result.Files))
	}
	if string(result.Summary) == "" {
		t.Error("expected non-empty summary")
	}
}

func TestExtractToDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extract.ctx")
	outDir := filepath.Join(dir, "output")

	files := map[string][]byte{
		schema.ManifestFile: []byte(`{"version":1,"created_at":"2024-01-01T00:00:00Z","tool":"ctx"}`),
		schema.MetadataFile: []byte(`{"project_name":"test","branch":"main","created_at":"2024-01-01T00:00:00Z","generator":"ctx","repository_root":"/tmp","os":"linux/amd64"}`),
		schema.GitFile:      []byte(`{"current_branch":"main","head_commit":"abc123","dirty":true}`),
		schema.SummaryFile:  []byte("## Current Objective\n\nTest\n"),
		schema.PromptsFile:  []byte(`[]`),
		schema.FilesFile:    []byte(`["main.go"]`),
		schema.PatchFile:    []byte("diff content"),
	}

	if err := Create(path, files); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := ExtractToDir(path, outDir); err != nil {
		t.Fatalf("ExtractToDir failed: %v", err)
	}

	for filename := range files {
		outPath := filepath.Join(outDir, filename)
		if _, err := os.Stat(outPath); err != nil {
			t.Errorf("expected file %s to exist in output dir", filename)
		}
	}
}

func TestSize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "size.ctx")

	files := map[string][]byte{
		schema.ManifestFile: []byte(`{"version":1}`),
		schema.MetadataFile: []byte(`{}`),
		schema.GitFile:      []byte(`{}`),
		schema.SummaryFile:  []byte("test"),
		schema.PromptsFile:  []byte(`[]`),
		schema.FilesFile:    []byte(`[]`),
		schema.PatchFile:    []byte(""),
	}

	if err := Create(path, files); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	size, err := Size(path)
	if err != nil {
		t.Fatalf("Size failed: %v", err)
	}
	if size <= 0 {
		t.Errorf("expected positive size, got %d", size)
	}
}

func TestIsZipFile_NonExistent(t *testing.T) {
	if IsZipFile("/nonexistent/file.ctx") {
		t.Error("expected false for nonexistent file")
	}
}

func TestIsZipFile_NotAZip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notzip.txt")
	if err := os.WriteFile(path, []byte("hello world"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if IsZipFile(path) {
		t.Error("expected false for non-ZIP file")
	}
}
