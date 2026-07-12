package providers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/RiddhiKatarki/ctx/pkg/types"
)

// openCodeProvider reads prompt history from OpenCode's
// prompt-history.jsonl file. OpenCode stores one JSON object per
// line with `input` (user message) and `parts` (may contain text).
type openCodeProvider struct {
	dir string
}

// NewOpenCodeProvider constructs an OpenCode prompt provider that
// reads from dir (defaults to ~/.local/state/opencode).
func NewOpenCodeProvider(dir string) PromptProvider {
	return &openCodeProvider{dir: dir}
}

func (p *openCodeProvider) defaultDir() string {
	if p.dir != "" {
		return p.dir
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local", "state", "opencode")
	}
	return ""
}

// Available reports whether the OpenCode prompt history file
// exists. Useful for auto-detection.
func (p *openCodeProvider) Available() bool {
	_, err := os.Stat(filepath.Join(p.defaultDir(), "prompt-history.jsonl"))
	return err == nil
}

// LastModified returns the mtime of the history file when available.
func (p *openCodeProvider) LastModified() (time.Time, bool) {
	fi, err := os.Stat(filepath.Join(p.defaultDir(), "prompt-history.jsonl"))
	if err != nil {
		return time.Time{}, false
	}
	return fi.ModTime(), true
}

func (p *openCodeProvider) History() ([]types.Prompt, error) {
	dir := p.defaultDir()
	if dir == "" {
		return nil, fmt.Errorf("opencode: cannot determine home directory")
	}

	path := filepath.Join(dir, "prompt-history.jsonl")
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opencode: failed to open %s: %w", path, err)
	}
	defer f.Close()

	var prompts []types.Prompt
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024) // accept large pasted lines

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry struct {
			Input string             `json:"input"`
			Parts []openCodePartItem `json:"parts"`
			Mode  string             `json:"mode"`
		}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue // skip malformed lines
		}

		content := strings.TrimSpace(entry.Input)
		if content == "" {
			// Some entries have content in `parts` instead.
			for _, part := range entry.Parts {
				if part.Text != "" {
					if content != "" {
						content += "\n\n"
					}
					content += strings.TrimSpace(part.Text)
				}
			}
		}
		if content == "" {
			continue
		}

		prompts = append(prompts, types.Prompt{
			Role:    "user",
			Content: truncateForContext(content, 4000),
		})
	}

	if err := scanner.Err(); err != nil {
		return prompts, fmt.Errorf("opencode: read %s: %w", path, err)
	}
	return prompts, nil
}

type openCodePartItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// truncateForContext caps very long pasted content so bundles
// stay readable. Keeps the head and tail of the content.
func truncateForContext(s string, max int) string {
	if len(s) <= max {
		return s
	}
	const marker = "\n\n[...truncated...]\n\n"
	half := (max - len(marker)) / 2
	return s[:half] + marker + s[len(s)-half:]
}
