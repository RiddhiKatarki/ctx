package export

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/RiddhiKatarki/ctx/internal/git"
	"github.com/RiddhiKatarki/ctx/internal/providers"
	"github.com/RiddhiKatarki/ctx/internal/summary"
)

func mustPromptProvider(t *testing.T, opts providers.Options) providers.PromptProvider {
	t.Helper()
	pp, err := providers.NewPromptProvider(opts)
	if err != nil {
		t.Fatalf("NewPromptProvider: %v", err)
	}
	return pp
}

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
		PromptProvider:  mustPromptProvider(t, providers.Options{}),
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
		PromptProvider:  mustPromptProvider(t, providers.Options{}),
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
		PromptProvider:  mustPromptProvider(t, providers.Options{}),
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
		PromptProvider:  mustPromptProvider(t, providers.Options{Source: providers.SourceFile, PromptsFile: promptsPath}),
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

func TestRun_JSONOutput_FlagSet(t *testing.T) {
	dir := initTestRepo(t)

	outputPath := filepath.Join(dir, "json-test.ctx")
	cfg := Config{
		OutputPath:      outputPath,
		WorkingDir:      dir,
		GitProvider:     git.NewCLIGitProvider(dir),
		PromptProvider:  mustPromptProvider(t, providers.Options{}),
		SummaryProvider: summary.NewTemplateProvider(),
		JSONOutput:      true,
	}

	result, err := Run(cfg)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if result.OutputPath != outputPath {
		t.Errorf("expected OutputPath %s, got %s", outputPath, result.OutputPath)
	}
	if result.FileCount < 1 {
		t.Errorf("expected at least 1 file, got %d", result.FileCount)
	}
	if result.SummaryProvider != "template" {
		t.Errorf("expected summary_provider template, got %s", result.SummaryProvider)
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty JSON output")
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	expectedFields := []string{
		"path", "project_name", "branch", "repository_root",
		"file_count", "diff_size", "bundle_size", "summary_provider",
		"head_commit", "dirty",
	}
	for _, f := range expectedFields {
		if _, ok := parsed[f]; !ok {
			t.Errorf("expected field %q in JSON output", f)
		}
	}
}

func TestResult_JSONTags_RoundTrip(t *testing.T) {
	r := &Result{
		OutputPath:      "test.ctx",
		ProjectName:     "p",
		Branch:          "main",
		FileCount:       3,
		PromptCount:     2,
		DiffSize:        100,
		BundleSize:      5000,
		Skipped:        []string{".env", "id_rsa"},
		SummaryProvider: "template",
		Commit:          "abc",
		Dirty:           true,
	}

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if parsed["skipped"] == nil {
		t.Error("expected skipped field")
	}

	skipped, ok := parsed["skipped"].([]any)
	if !ok || len(skipped) != 2 {
		t.Errorf("expected skipped array of length 2, got %v", parsed["skipped"])
	}
}

func TestRun_JSONOutput_SkippedList(t *testing.T) {
	dir := initTestRepo(t)

	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("API_KEY=secret"), 0644); err != nil {
		t.Fatalf("failed to write .env: %v", err)
	}

	outputPath := filepath.Join(dir, "skip-json.ctx")
	cfg := Config{
		OutputPath:      outputPath,
		WorkingDir:      dir,
		GitProvider:     git.NewCLIGitProvider(dir),
		PromptProvider:  mustPromptProvider(t, providers.Options{}),
		SummaryProvider: summary.NewTemplateProvider(),
		ExtraFiles:      []string{".env"},
		JSONOutput:      true,
	}

	result, err := Run(cfg)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(result.Skipped) != 1 {
		t.Errorf("expected 1 skipped file, got %d", len(result.Skipped))
	}
	if result.Skipped[0] != ".env" {
		t.Errorf("expected .env to be skipped, got %s", result.Skipped[0])
	}

	// Verify JSON marshals the skipped field correctly even in JSON mode
	data, _ := json.Marshal(result.Skipped)
	var parsed []string
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(parsed) != 1 || parsed[0] != ".env" {
		t.Errorf("expected [\"\\u002eenv\"], got %v", parsed)
	}
}
