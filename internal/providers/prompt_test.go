package providers

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMockPromptProvider_ReturnsEmpty(t *testing.T) {
	p := &MockPromptProvider{}
	prompts, err := p.History()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prompts) != 0 {
		t.Errorf("expected 0 prompts, got %d", len(prompts))
	}
}

func TestFilePromptProvider_ValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prompts.json")
	content := `[
		{"role":"user","content":"Implement retry logic."},
		{"role":"assistant","content":"I'll add a retry wrapper."}
	]`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	p := &FilePromptProvider{path: path}
	prompts, err := p.History()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prompts) != 2 {
		t.Fatalf("expected 2 prompts, got %d", len(prompts))
	}
	if prompts[0].Role != "user" {
		t.Errorf("expected role 'user', got %s", prompts[0].Role)
	}
	if prompts[0].Content != "Implement retry logic." {
		t.Errorf("unexpected content: %s", prompts[0].Content)
	}
}

func TestFilePromptProvider_FileNotFound(t *testing.T) {
	p := &FilePromptProvider{path: "/nonexistent/prompts.json"}
	_, err := p.History()
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestFilePromptProvider_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prompts.json")
	if err := os.WriteFile(path, []byte("{invalid json"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	p := &FilePromptProvider{path: path}
	_, err := p.History()
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestFilePromptProvider_InvalidRole(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prompts.json")
	content := `[{"role":"invalid","content":"test"}]`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	p := &FilePromptProvider{path: path}
	_, err := p.History()
	if err == nil {
		t.Error("expected error for invalid role")
	}
}

func TestNewPromptProvider_Mock(t *testing.T) {
	p := NewPromptProvider(Options{})
	if _, ok := p.(*MockPromptProvider); !ok {
		t.Error("expected MockPromptProvider when no file specified")
	}
}

func TestNewPromptProvider_File(t *testing.T) {
	p := NewPromptProvider(Options{PromptsFile: "prompts.json"})
	if _, ok := p.(*FilePromptProvider); !ok {
		t.Error("expected FilePromptProvider when file specified")
	}
}
