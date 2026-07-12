package git

import (
	"errors"
	"testing"
)

var errNotFound = errors.New("command not found in mock")

// mockRunner implements gitRunner for testing.
type mockRunner struct {
	responses map[string]string
	byteResps map[string][]byte
	errs      map[string]error
}

func (m *mockRunner) Run(args ...string) (string, error) {
	key := joinArgs(args)
	if err, ok := m.errs[key]; ok {
		return "", err
	}
	if resp, ok := m.responses[key]; ok {
		return resp, nil
	}
	return "", errNotFound
}

func (m *mockRunner) RunBytes(args ...string) ([]byte, error) {
	key := joinArgs(args)
	if err, ok := m.errs[key]; ok {
		return nil, err
	}
	if resp, ok := m.byteResps[key]; ok {
		return resp, nil
	}
	return nil, errNotFound
}

func joinArgs(args []string) string {
	result := ""
	for i, a := range args {
		if i > 0 {
			result += " "
		}
		result += a
	}
	return result
}

func TestCLIGitProvider_IsRepository_True(t *testing.T) {
	p := &CLIGitProvider{runner: &mockRunner{
		responses: map[string]string{
			"rev-parse --git-dir": ".git",
		},
	}}
	if !p.IsRepository() {
		t.Error("expected IsRepository to return true")
	}
}

func TestCLIGitProvider_IsRepository_False(t *testing.T) {
	p := &CLIGitProvider{runner: &mockRunner{}}
	if p.IsRepository() {
		t.Error("expected IsRepository to return false")
	}
}

func TestCLIGitProvider_RepositoryRoot(t *testing.T) {
	p := &CLIGitProvider{runner: &mockRunner{
		responses: map[string]string{
			"rev-parse --show-toplevel": "/home/user/project",
		},
	}}
	root, err := p.RepositoryRoot()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root != "/home/user/project" {
		t.Errorf("expected /home/user/project, got %s", root)
	}
}

func TestCLIGitProvider_Metadata(t *testing.T) {
	p := &CLIGitProvider{runner: &mockRunner{
		responses: map[string]string{
			"rev-parse --abbrev-ref HEAD":           "feature/test",
			"rev-parse HEAD":                        "abc123def456",
			"status --porcelain":                    "M file.go\n",
			"remote get-url origin":                 "git@github.com:user/repo.git",
			"describe --tags --exact-match HEAD":    "v1.0.0",
		},
	}}

	meta, err := p.Metadata()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if meta.CurrentBranch != "feature/test" {
		t.Errorf("expected branch feature/test, got %s", meta.CurrentBranch)
	}
	if meta.HeadCommit != "abc123def456" {
		t.Errorf("expected HEAD abc123def456, got %s", meta.HeadCommit)
	}
	if !meta.Dirty {
		t.Error("expected dirty=true")
	}
	if meta.RemoteURL != "git@github.com:user/repo.git" {
		t.Errorf("expected remote URL, got %s", meta.RemoteURL)
	}
	if meta.CurrentTag != "v1.0.0" {
		t.Errorf("expected tag v1.0.0, got %s", meta.CurrentTag)
	}
}

func TestCLIGitProvider_Metadata_Clean(t *testing.T) {
	p := &CLIGitProvider{runner: &mockRunner{
		responses: map[string]string{
			"rev-parse --abbrev-ref HEAD": "main",
			"rev-parse HEAD":              "abc123",
			"status --porcelain":          "",
		},
	}}

	meta, err := p.Metadata()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if meta.Dirty {
		t.Error("expected dirty=false for clean repo")
	}
}

func TestCLIGitProvider_Metadata_Error(t *testing.T) {
	p := &CLIGitProvider{runner: &mockRunner{}}
	_, err := p.Metadata()
	if err == nil {
		t.Error("expected error when git commands fail")
	}
}

func TestCLIGitProvider_Diff(t *testing.T) {
	p := &CLIGitProvider{runner: &mockRunner{
		byteResps: map[string][]byte{
			"diff HEAD": []byte("diff --git a/file.go b/file.go\n+new line"),
		},
	}}

	diff, err := p.Diff()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(diff) != "diff --git a/file.go b/file.go\n+new line" {
		t.Errorf("unexpected diff content: %s", string(diff))
	}
}

func TestCLIGitProvider_ModifiedFiles(t *testing.T) {
	p := &CLIGitProvider{runner: &mockRunner{
		responses: map[string]string{
			"diff --name-only HEAD": "main.go\nauth.go\nREADME.md\n",
		},
	}}

	files, err := p.ModifiedFiles()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(files))
	}
	if files[0] != "main.go" {
		t.Errorf("expected main.go, got %s", files[0])
	}
}

func TestCLIGitProvider_ModifiedFiles_Empty(t *testing.T) {
	p := &CLIGitProvider{runner: &mockRunner{
		responses: map[string]string{
			"diff --name-only HEAD": "",
		},
	}}

	files, err := p.ModifiedFiles()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}
