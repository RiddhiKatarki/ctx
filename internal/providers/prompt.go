package providers

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/context-handoff/ctx/pkg/types"
)

// PromptProvider is the interface for retrieving prompt history.
// Future implementations may integrate with Claude Code, Cursor,
// OpenCode, Windsurf, Aider, etc.
type PromptProvider interface {
	History() ([]types.Prompt, error)
}

// Options configures which prompt provider to use.
type Options struct {
	// PromptsFile is the path to a JSON file with prompt history.
	// If empty, the mock provider is used.
	PromptsFile string
}

// NewPromptProvider selects an implementation based on the given options.
func NewPromptProvider(opts Options) PromptProvider {
	if opts.PromptsFile != "" {
		return &FilePromptProvider{path: opts.PromptsFile}
	}
	return &MockPromptProvider{}
}

// MockPromptProvider returns an empty prompt history.
// It is the default when no prompt source is configured.
type MockPromptProvider struct{}

func (p *MockPromptProvider) History() ([]types.Prompt, error) {
	return []types.Prompt{}, nil
}

// FilePromptProvider reads prompt history from a JSON file.
// The file must contain an array of {role, content} objects.
type FilePromptProvider struct {
	path string
}

func (p *FilePromptProvider) History() ([]types.Prompt, error) {
	data, err := os.ReadFile(p.path)
	if err != nil {
		return nil, fmt.Errorf("failed to read prompts file %s: %w", p.path, err)
	}

	var prompts []types.Prompt
	if err := json.Unmarshal(data, &prompts); err != nil {
		return nil, fmt.Errorf("failed to parse prompts file %s: %w", p.path, err)
	}

	for i, pr := range prompts {
		if pr.Role != "user" && pr.Role != "assistant" && pr.Role != "system" {
			return nil, fmt.Errorf("prompts[%d]: invalid role %q (must be user, assistant, or system)", i, pr.Role)
		}
	}

	return prompts, nil
}
