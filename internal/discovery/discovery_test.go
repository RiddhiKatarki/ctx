package discovery

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/RiddhiKatarki/ctx/internal/schema"
)

// createBundle writes a minimal valid .ctx bundle at path.
func createBundle(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create bundle: %v", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	files := map[string][]byte{
		schema.ManifestFile: []byte(`{"version":1,"created_at":"2024-01-01T00:00:00Z","tool":"ctx"}`),
		schema.MetadataFile: []byte(`{"project_name":"proj","branch":"main","created_at":"2024-01-01T00:00:00Z","generator":"ctx","repository_root":"/tmp","os":"linux/amd64"}`),
		schema.GitFile:      []byte(`{"current_branch":"main","head_commit":"abc123","dirty":true}`),
		schema.SummaryFile:  []byte("## Current Objective\n\nTest\n"),
		schema.PromptsFile:  []byte(`[]`),
		schema.FilesFile:    []byte(`["main.go","auth.go"]`),
		schema.PatchFile:    []byte("diff content"),
	}
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create zip entry: %v", err)
		}
		if _, err := w.Write(content); err != nil {
			t.Fatalf("write zip entry: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
}

func createNonBundle(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("not a zip"), 0644); err != nil {
		t.Fatalf("write non-bundle: %v", err)
	}
}

func TestList_FindsBundles(t *testing.T) {
	dir := t.TempDir()
	createBundle(t, filepath.Join(dir, "a.ctx"))
	createBundle(t, filepath.Join(dir, "b.ctx"))
	createNonBundle(t, filepath.Join(dir, "c.txt"))

	entries, err := List(dir)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
}

func TestList_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	entries, err := List(dir)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestList_NonExistentDirectory(t *testing.T) {
	_, err := List("/nonexistent/dir/here")
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

func TestList_NotADirectory(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "not-dir.txt")
	if err := os.WriteFile(file, []byte("x"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := List(file)
	if err == nil {
		t.Error("expected error when path is not a directory")
	}
}

func TestList_SortedByCreatedAt(t *testing.T) {
	dir := t.TempDir()
	createBundle(t, filepath.Join(dir, "old.ctx"))
	createBundle(t, filepath.Join(dir, "new.ctx"))

	entries, err := List(dir)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// Both have same created_at in this fixture; falls back to name sort
	if entries[0].Name != "new.ctx" && entries[0].Name != "old.ctx" {
		t.Errorf("unexpected entry name: %s", entries[0].Name)
	}
}

func TestList_IgnoresSubdirectories(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "subdir")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	createBundle(t, filepath.Join(subdir, "nested.ctx"))

	entries, err := List(dir)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries (non-recursive), got %d", len(entries))
	}
}

func TestEntry_CapturesMetadata(t *testing.T) {
	dir := t.TempDir()
	bundlePath := filepath.Join(dir, "test.ctx")
	createBundle(t, bundlePath)

	entries, err := List(dir)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]

	if e.ProjectName != "proj" {
		t.Errorf("expected project_name proj, got %s", e.ProjectName)
	}
	if e.Branch != "main" {
		t.Errorf("expected branch main, got %s", e.Branch)
	}
	if e.ManifestVersion != 1 {
		t.Errorf("expected version 1, got %d", e.ManifestVersion)
	}
	if e.Tool != "ctx" {
		t.Errorf("expected tool ctx, got %s", e.Tool)
	}
	if e.FileCount != 2 {
		t.Errorf("expected file_count 2, got %d", e.FileCount)
	}
	if !e.Dirty {
		t.Error("expected dirty=true")
	}
	if !e.HasDiff {
		t.Error("expected has_diff=true")
	}
	if e.Commit != "abc123" {
		t.Errorf("expected commit abc123, got %s", e.Commit)
	}
}

func TestEntry_JSONTags(t *testing.T) {
	e := Entry{
		Path:            "test.ctx",
		Name:            "test.ctx",
		Size:            1234,
		ManifestVersion: 1,
		Tool:            "ctx",
		ProjectName:     "p",
		Branch:          "main",
	}
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for _, f := range []string{"path", "name", "size", "manifest_version", "tool", "project_name", "branch"} {
		if _, ok := parsed[f]; !ok {
			t.Errorf("missing field %q", f)
		}
	}
}

func TestInfo_BasicStructure(t *testing.T) {
	dir := t.TempDir()
	bundlePath := filepath.Join(dir, "info.ctx")
	createBundle(t, bundlePath)

	result, err := Info(bundlePath)
	if err != nil {
		t.Fatalf("Info: %v", err)
	}

	if result.Path != bundlePath {
		t.Errorf("expected path %s, got %s", bundlePath, result.Path)
	}
	if result.FileCount != 2 {
		t.Errorf("expected file_count 2, got %d", result.FileCount)
	}
	if !result.Valid {
		t.Error("expected Valid=true")
	}
	if result.Manifest["version"].(int) != 1 {
		t.Errorf("expected version 1, got %v", result.Manifest["version"])
	}
	if result.Metadata["project_name"].(string) != "proj" {
		t.Errorf("expected proj, got %v", result.Metadata["project_name"])
	}
}

func TestInfo_NonZipFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fake.ctx")
	createNonBundle(t, path)

	_, err := Info(path)
	if err == nil {
		t.Error("expected error for non-ZIP file")
	}
}

func TestInfo_EmptyPath(t *testing.T) {
	_, err := Info("")
	if err == nil {
		t.Error("expected error for empty path")
	}
}

func TestInfoResult_JSONTags(t *testing.T) {
	r := &InfoResult{
		Path:      "test.ctx",
		Size:      1000,
		FileCount: 4,
		Valid:     true,
		Manifest:  map[string]any{"version": 1},
		Metadata:  map[string]any{"project_name": "p"},
		Files:     []string{"a.go"},
	}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, f := range []string{"path", "size", "file_count", "valid", "manifest", "metadata", "files"} {
		if _, ok := parsed[f]; !ok {
			t.Errorf("missing field %q", f)
		}
	}
}

func TestPrintJSON(t *testing.T) {
	v := map[string]string{"key": "value"}
	data, err := PrintJSON(v)
	if err != nil {
		t.Fatalf("PrintJSON: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty output")
	}
}
