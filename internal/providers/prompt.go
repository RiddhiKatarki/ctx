package providers

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/RiddhiKatarki/ctx/pkg/types"
)

// PromptProvider is the interface for retrieving prompt history.
type PromptProvider interface {
	History() ([]types.Prompt, error)
}

// AvailChecker is an optional interface implemented by providers
// that can report whether they have data and when it was last updated.
// The factory uses it for source=auto detection.
type AvailChecker interface {
	Available() bool
	LastModified() (time.Time, bool)
}

// Source identifies a prompt provider by name. The factory accepts
// these strings via the --prompts-source flag.
type Source string

const (
	SourceAuto     Source = "auto"
	SourceFile     Source = "file"
	SourceClaude   Source = "claudecode"
	SourceOpenCode Source = "opencode"
	SourceCursor   Source = "cursor"
	SourceAider    Source = "aider"
	SourceNone     Source = "none"
	SourceMock     Source = "mock"
)

// Options configures NewPromptProvider.
type Options struct {
	// Source selects the prompt provider explicitly. When empty
	// (or "auto"), the factory probes auto-detected providers in
	// order and picks the freshest.
	Source Source

	// PromptsFile selects the file-based provider. Mutually exclusive
	// with SourceClaude/OpenCode/Cursor/Aider.
	PromptsFile string

	// WorkingDir is an optional hint for source lookup
	// (e.g. used to scope the Aider project-local history file).
	WorkingDir string

	// CacheDir / ClaudeDir / etc are provider-specific override paths
	// useful for testing or non-default install locations.
	OverrideDirs map[string]string
}

// NewPromptProvider returns the prompt provider matching opts.Source.
// Defaults to a MockPromptProvider when no data is found.
func NewPromptProvider(opts Options) (PromptProvider, error) {
	src := opts.Source
	if src == "" {
		src = SourceAuto
	}

	dirs := opts.OverrideDirs

	switch src {
	case SourceAuto:
		return autoDetectProvider(opts.WorkingDir, dirs)
	case SourceFile:
		if opts.PromptsFile == "" {
			return nil, errors.New("file source requires --prompts")
		}
		return &FilePromptProvider{path: opts.PromptsFile}, nil
	case SourceClaude:
		return NewClaudeCodeProvider(dirOr(dirs, "claudecode", ".claude"), opts.WorkingDir), nil
	case SourceOpenCode:
		return NewOpenCodeProvider(dirOr(dirs, "opencode", defaultOpenCodeDir())), nil
	case SourceCursor:
		return NewCursorProvider(dirOr(dirs, "cursor", ""), opts.WorkingDir), nil
	case SourceAider:
		return NewAiderProvider(dirOr(dirs, "aider", ""), opts.WorkingDir), nil
	case SourceNone, SourceMock:
		return &MockPromptProvider{}, nil
	default:
		return nil, fmt.Errorf("unknown prompt source: %q", src)
	}
}

// autoDetectProvider probes each provider in priority order and
// returns the freshest one. Falls back to MockPromptProvider when
// nothing has usable data.
func autoDetectProvider(cwd string, dirs map[string]string) (PromptProvider, error) {
	probes := []PromptProvider{
		NewOpenCodeProvider(dirOr(dirs, "opencode", defaultOpenCodeDir())),
		NewClaudeCodeProvider(dirOr(dirs, "claudecode", ".claude"), cwd),
		NewCursorProvider(dirOr(dirs, "cursor", ""), cwd),
		NewAiderProvider(dirOr(dirs, "aider", ""), cwd),
	}

	var freshest PromptProvider
	var freshestTime time.Time

	for _, p := range probes {
		ac, ok := p.(AvailChecker)
		if !ok || !ac.Available() {
			continue
		}
		t, hasTime := ac.LastModified()
		if !hasTime {
			continue
		}
		if freshest == nil || t.After(freshestTime) {
			freshest = p
			freshestTime = t
		}
	}

	if freshest != nil {
		return freshest, nil
	}
	return &MockPromptProvider{}, nil
}

func dirOr(dirs map[string]string, key, fallback string) string {
	if dirs != nil {
		if v, ok := dirs[key]; ok && v != "" {
			return v
		}
	}
	return fallback
}

func defaultOpenCodeDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local", "state", "opencode")
	}
	return ""
}

// MockPromptProvider returns an empty prompt history.
// It is the default when no prompt source is configured or when
// auto-detection finds nothing usable.
type MockPromptProvider struct{}

func (p *MockPromptProvider) History() ([]types.Prompt, error) {
	return []types.Prompt{}, nil
}

// Available always returns false so auto-detection skips Mock.
func (p *MockPromptProvider) Available() bool                 { return false }
func (p *MockPromptProvider) LastModified() (time.Time, bool) { return time.Time{}, false }

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
