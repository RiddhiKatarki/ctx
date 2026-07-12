package git

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/context-handoff/ctx/pkg/types"
)

// GitProvider is the interface for retrieving Git state.
// Implementations may use the Git CLI, go-git, or any other backend.
type GitProvider interface {
	Metadata() (*types.GitMetadata, error)
	Diff() ([]byte, error)
	ModifiedFiles() ([]string, error)
	IsRepository() bool
	RepositoryRoot() (string, error)
}

// gitRunner is the seam that isolates exec calls for mocking.
type gitRunner interface {
	Run(args ...string) (stdout string, err error)
	RunBytes(args ...string) (stdout []byte, err error)
}

// CLIGitProvider implements GitProvider by invoking the git binary.
type CLIGitProvider struct {
	runner gitRunner
}

// execRunner is the production gitRunner using os/exec.
type execRunner struct {
	dir string
}

func (r *execRunner) Run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (r *execRunner) RunBytes(args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.dir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

// NewCLIGitProvider creates a GitProvider that invokes the git CLI
// from the given working directory.
func NewCLIGitProvider(dir string) GitProvider {
	return &CLIGitProvider{runner: &execRunner{dir: dir}}
}

func (p *CLIGitProvider) IsRepository() bool {
	_, err := p.runner.Run("rev-parse", "--git-dir")
	return err == nil
}

func (p *CLIGitProvider) RepositoryRoot() (string, error) {
	root, err := p.runner.Run("rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return root, nil
}

func (p *CLIGitProvider) Metadata() (*types.GitMetadata, error) {
	branch, err := p.runner.Run("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("failed to get current branch: %w", err)
	}

	head, err := p.runner.Run("rev-parse", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD commit: %w", err)
	}

	status, err := p.runner.Run("status", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("failed to get repository status: %w", err)
	}
	dirty := strings.TrimSpace(status) != ""

	remoteURL, _ := p.runner.Run("remote", "get-url", "origin")

	currentTag, _ := p.runner.Run("describe", "--tags", "--exact-match", "HEAD")

	return &types.GitMetadata{
		CurrentBranch: branch,
		HeadCommit:    head,
		Dirty:         dirty,
		RemoteURL:     remoteURL,
		CurrentTag:    currentTag,
	}, nil
}

func (p *CLIGitProvider) Diff() ([]byte, error) {
	out, err := p.runner.RunBytes("diff", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("failed to get git diff: %w", err)
	}
	return out, nil
}

func (p *CLIGitProvider) ModifiedFiles() ([]string, error) {
	out, err := p.runner.Run("diff", "--name-only", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("failed to get modified files: %w", err)
	}
	var files []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}
