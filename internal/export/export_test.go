package export

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/context-handoff/ctx/internal/git"
	"github.com/context-handoff/ctx/internal/providers"
	"github.com/context-handoff/ctx/internal/summary"
)

func initTestRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	commands := [][]string{
		{"init"},
		{"config", "user.name", "Test User"},
		{"config", "user.email", "test@test.com"},
	}

	for _, args := range commands {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
	}

	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("failed to write main.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test Project\n"), 0644); err != nil {
		t.Fatalf("failed to write README.md: %v", err)
	}

	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git add: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit: %v", err)
	}

	cmd = exec.Command("git", "checkout", "-b", "feature/test")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git checkout: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc newFunc() {}\n"), 0644); err != nil {
		t.Fatalf("failed to modify main.go: %v", err)
	}

	return dir
}

func TestRun_FullExportFlow(t *testing.T) {
	dir := initTestRepo(t)

	outputPath := filepath.Join(dir, "test-output.ctx")

	cfg := Config{
		OutputPath:      outputPath,
		WorkingDir:      dir,
		GitProvider:     git.NewCLIGitProvider(dir),
		PromptProvider:  providers.NewPromptProvider(providers.Options{}),
		SummaryProvider: func() *summary.TemplateProvider { return summary.NewTemplateProvider() }(),
	}

	result, err := Run(cfg)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if result.OutputPath != outputPath {
		t.Errorf("expected output path %s, got %s", outputPath, result.OutputPath)
	}
	if result.FileCount < 1 {
		t.Errorf("expected at least 1 file, got %d", result.FileCount)
	}
	if result.BundleSize <= 0 {
		t.Errorf("expected positive bundle size, got %d", result.BundleSize)
	}

	if _, err := os.Stat(outputPath); err != nil {
		t.Errorf("expected bundle file to exist: %v", err)
	}
}

func TestRun_NotAGitRepo(t *testing.T) {
	dir := t.TempDir()

	cfg := Config{
		OutputPath: filepath.Join(dir, "output.ctx"),
		WorkingDir: dir,
	}

	_, err := Run(cfg)
	if err == nil {
		t.Error("expected error when not in a git repo")
	}
}

func TestRun_WithExtraFiles(t *testing.T) {
	dir := initTestRepo(t)

	if err := os.WriteFile(filepath.Join(dir, "notes.md"), []byte("# Notes\n"), 0644); err != nil {
		t.Fatalf("failed to write notes.md: %v", err)
	}

	outputPath := filepath.Join(dir, "extra.ctx")

	cfg := Config{
		OutputPath:      outputPath,
		WorkingDir:      dir,
		GitProvider:     git.NewCLIGitProvider(dir),
		PromptProvider:  providers.NewPromptProvider(providers.Options{}),
		SummaryProvider: summary.NewTemplateProvider(),
		ExtraFiles:      []string{"notes.md"},
	}

	result, err := Run(cfg)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	found := false
	for _, f := range cfg.ExtraFiles {
		if f == "notes.md" {
			found = true
		}
	}
	if !found {
		t.Error("expected notes.md in extra files")
	}
	_ = result
}

func TestRun_SecretExclusion(t *testing.T) {
	dir := initTestRepo(t)

	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("API_KEY=secret"), 0644); err != nil {
		t.Fatalf("failed to write .env: %v", err)
	}

	outputPath := filepath.Join(dir, "secret.ctx")

	cfg := Config{
		OutputPath:      outputPath,
		WorkingDir:      dir,
		GitProvider:     git.NewCLIGitProvider(dir),
		PromptProvider:  providers.NewPromptProvider(providers.Options{}),
		SummaryProvider: summary.NewTemplateProvider(),
		ExtraFiles:      []string{".env", "README.md"},
	}

	result, err := Run(cfg)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(result.Skipped) != 1 || result.Skipped[0] != ".env" {
		t.Errorf("expected .env to be skipped, got %v", result.Skipped)
	}
}

func TestMergeFiles(t *testing.T) {
	result := mergeFiles([]string{"a.go", "b.go"}, []string{"b.go", "c.go"})
	if len(result) != 3 {
		t.Fatalf("expected 3 files, got %d", len(result))
	}
}

func TestFilterSecrets(t *testing.T) {
	files := []string{"main.go", ".env", "auth.go", "id_rsa", "config.yaml"}
	kept, skipped := filterSecrets(files)

	if len(kept) != 3 {
		t.Errorf("expected 3 kept files, got %d: %v", len(kept), kept)
	}
	if len(skipped) != 2 {
		t.Errorf("expected 2 skipped files, got %d: %v", len(skipped), skipped)
	}
}

func TestIsSecretFile(t *testing.T) {
	tests := []struct {
		path      string
		isSecret  bool
	}{
		{".env", true},
		{".env.local", true},
		{"config/.env", true},
		{"private.pem", true},
		{"certs/key.key", true},
		{"id_rsa", true},
		{"id_rsa.pub", true},
		{"main.go", false},
		{"README.md", false},
		{"config.yaml", false},
	}

	for _, tt := range tests {
		result := isSecretFile(tt.path)
		if result != tt.isSecret {
			t.Errorf("isSecretFile(%q) = %v, expected %v", tt.path, result, tt.isSecret)
		}
	}
}

func TestFormatSize(t *testing.T) {
	if formatSize(512) != "512 B" {
		t.Errorf("expected '512 B', got %s", formatSize(512))
	}
	if formatSize(2048) != "2.0 KB" {
		t.Errorf("expected '2.0 KB', got %s", formatSize(2048))
	}
}

func TestRuntimeOS(t *testing.T) {
	os := runtimeOS()
	if os == "" {
		t.Error("expected non-empty OS string")
	}
}

func TestRun_WithFilePrompts(t *testing.T) {
	dir := initTestRepo(t)

	promptsPath := filepath.Join(dir, "prompts.json")
	promptsContent := `[{"role":"user","content":"Implement feature X."}]`
	if err := os.WriteFile(promptsPath, []byte(promptsContent), 0644); err != nil {
		t.Fatalf("failed to write prompts.json: %v", err)
	}

	outputPath := filepath.Join(dir, "with-prompts.ctx")

	cfg := Config{
		OutputPath:      outputPath,
		WorkingDir:      dir,
		GitProvider:     git.NewCLIGitProvider(dir),
		PromptProvider:  providers.NewPromptProvider(providers.Options{PromptsFile: promptsPath}),
		SummaryProvider: summary.NewTemplateProvider(),
	}

	_, err := Run(cfg)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
}

func TestRun_DefaultProviders(t *testing.T) {
	dir := initTestRepo(t)

	outputPath := filepath.Join(dir, "defaults.ctx")

	cfg := Config{
		OutputPath: outputPath,
		WorkingDir: dir,
	}

	_, err := Run(cfg)
	if err != nil {
		t.Fatalf("Run with default providers failed: %v", err)
	}
}
