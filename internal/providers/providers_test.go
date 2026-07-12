package providers

import (
	"os"
	"path/filepath"
	"testing"
)

// writeFile is a test helper that writes file content with permissions.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestOpenCodeProvider_History(t *testing.T) {
	dir := t.TempDir()
	histPath := filepath.Join(dir, "prompt-history.jsonl")
	content := `{"input":"Implement retry logic","parts":[],"mode":"normal"}
{"input":"Add tests for it","parts":[],"mode":"shell"}
`
	writeFile(t, histPath, content)

	p := NewOpenCodeProvider(dir)
	prompts, err := p.History()
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(prompts) != 2 {
		t.Fatalf("expected 2 prompts, got %d", len(prompts))
	}
	if prompts[0].Role != "user" {
		t.Errorf("expected role 'user', got %s", prompts[0].Role)
	}
	if prompts[0].Content != "Implement retry logic" {
		t.Errorf("unexpected content: %s", prompts[0].Content)
	}
}

func TestOpenCodeProvider_Available(t *testing.T) {
	dir := t.TempDir()
	p := NewOpenCodeProvider(dir)
	if p.(AvailChecker).Available() {
		t.Error("expected not available with empty dir")
	}

	writeFile(t, filepath.Join(dir, "prompt-history.jsonl"), `{"input":"x"}`)
	if !p.(AvailChecker).Available() {
		t.Error("expected available after creating file")
	}
}

func TestOpenCodeProvider_TruncatesLongInput(t *testing.T) {
	dir := t.TempDir()
	long := make([]byte, 8000)
	for i := range long {
		long[i] = 'x'
	}
	writeFile(t, filepath.Join(dir, "prompt-history.jsonl"),
		`{"input":"`+string(long)+`"}`)

	p := NewOpenCodeProvider(dir)
	prompts, err := p.History()
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(prompts) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(prompts))
	}
	if len(prompts[0].Content) > 4010 {
		t.Errorf("expected content <= 4010 chars after truncation, got %d", len(prompts[0].Content))
	}
	if !contains(prompts[0].Content, "[...truncated...]") {
		t.Error("expected truncation marker")
	}
}

func TestClaudeCodeProvider_History(t *testing.T) {
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "projects", "-root-ctx")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	sessionContent := `{"type":"user","message":{"role":"user","content":"Hello, please help me debug"}}
{"type":"assistant","message":{"role":"assistant","content":"Sure, what's the issue?"}}
{"type":"user","message":{"role":"user","content":[{"type":"text","text":"Multiple line content"}]}}
`
	writeFile(t, filepath.Join(projectDir, "session.jsonl"), sessionContent)

	p := NewClaudeCodeProvider(dir, "")
	prompts, err := p.History()
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(prompts) != 3 {
		t.Fatalf("expected 3 prompts, got %d", len(prompts))
	}
	if prompts[0].Role != "user" {
		t.Errorf("expected role 'user', got %s", prompts[0].Role)
	}
	if prompts[1].Role != "assistant" {
		t.Errorf("expected role 'assistant', got %s", prompts[1].Role)
	}
	if prompts[2].Content != "Multiple line content" {
		t.Errorf("expected content from array format, got %q", prompts[2].Content)
	}
}

func TestClaudeCodeProvider_Available(t *testing.T) {
	dir := t.TempDir()
	p := NewClaudeCodeProvider(dir, "")
	if p.(AvailChecker).Available() {
		t.Error("expected not available with empty dir")
	}

	projectDir := filepath.Join(dir, "projects", "x")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, filepath.Join(projectDir, "session.jsonl"), "{}")
	if !p.(AvailChecker).Available() {
		t.Error("expected available after creating session file")
	}
}

func TestCursorProvider_Detection(t *testing.T) {
	dir := t.TempDir()
	// Require User/ subdirectory for valid install.
	if err := os.MkdirAll(filepath.Join(dir, "User"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	p := NewCursorProvider(dir)
	if !p.(AvailChecker).Available() {
		t.Error("expected available when User/ subdir exists")
	}

	// Empty dir without User/ → not available
	empty := t.TempDir()
	p2 := NewCursorProvider(empty)
	if p2.(AvailChecker).Available() {
		t.Error("expected not available when User/ subdir missing")
	}
}

func TestCursorProvider_NoInstall(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "no-cursor-here")
	p := NewCursorProvider(dir)
	if p.(AvailChecker).Available() {
		t.Error("expected not available for missing install dir")
	}
}

func TestCursorProvider_History_Limited(t *testing.T) {
	dir := t.TempDir()
	p := NewCursorProvider(dir)
	_, err := p.History()
	if err == nil {
		t.Error("expected limited-error indicating SQLite not implemented")
	}
	if !contains(err.Error(), "SQLite") {
		t.Errorf("expected error to mention SQLite, got %v", err)
	}
}

func TestAiderProvider_PlainHistory(t *testing.T) {
	dir := t.TempDir()
	content := `USER: Add user authentication.
ASSISTANT: I'll add bcrypt hashing and session tokens.
USER: Also add tests.
ASSISTANT: Adding tests now.
`
	writeFile(t, filepath.Join(dir, ".aider.chat.history"), content)

	p := NewAiderProvider(dir, "")
	prompts, err := p.History()
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(prompts) != 4 {
		t.Fatalf("expected 4 prompts, got %d", len(prompts))
	}
	if prompts[0].Role != "user" {
		t.Errorf("expected 'user', got %s", prompts[0].Role)
	}
	if prompts[1].Role != "assistant" {
		t.Errorf("expected 'assistant', got %s", prompts[1].Role)
	}
	if prompts[0].Content != "Add user authentication." {
		t.Errorf("unexpected content: %s", prompts[0].Content)
	}
}

func TestAiderProvider_MarkdownHistory(t *testing.T) {
	dir := t.TempDir()
	cwd := t.TempDir()
	content := `# aider chat started at 2024-01-01

#### user
Refactor the auth module.

#### assistant
Split out auth into a separate package.

#### user
Add tests for it.
`
	writeFile(t, filepath.Join(cwd, ".aider.chat.history.md"), content)

	p := NewAiderProvider(dir, cwd)
	prompts, err := p.History()
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(prompts) != 3 {
		t.Fatalf("expected 3 prompts, got %d", len(prompts))
	}
	if prompts[0].Role != "user" {
		t.Errorf("expected 'user', got %s", prompts[0].Role)
	}
	if prompts[0].Content != "Refactor the auth module." {
		t.Errorf("unexpected: %s", prompts[0].Content)
	}
	if prompts[1].Role != "assistant" {
		t.Errorf("expected 'assistant', got %s", prompts[1].Role)
	}
}

func TestAiderProvider_AvailableDetection(t *testing.T) {
	// No install → not available
	dir := filepath.Join(t.TempDir(), "no-aider")
	p := NewAiderProvider(dir, "")
	if p.(AvailChecker).Available() {
		t.Error("expected not available when no chat history exists")
	}

	// With global history → available
	dir = t.TempDir()
	writeFile(t, filepath.Join(dir, ".aider.chat.history"), "USER: hi")
	p = NewAiderProvider(dir, "")
	if !p.(AvailChecker).Available() {
		t.Error("expected available with global history")
	}
}

func TestNewPromptProvider_AutoDetectsRealData(t *testing.T) {
	if !hasOpenCodeData() {
		t.Skip("no opencode data on this host")
	}

	p, err := NewPromptProvider(Options{})
	if err != nil {
		t.Fatalf("auto detect: %v", err)
	}
	if _, ok := p.(*openCodeProvider); !ok {
		t.Errorf("expected openCodeProvider (auto-detected), got %T", p)
	}
}

func hasOpenCodeData() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(home, ".local", "state", "opencode", "prompt-history.jsonl"))
	return err == nil
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
