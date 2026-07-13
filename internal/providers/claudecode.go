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

// claudeCodeProvider reads prompt history from Claude Code's
// JSONL session files under ~/.claude/projects/<encoded-cwd>/.
// Each line is a message record with `type` and `message.role` + `message.content`.
type claudeCodeProvider struct {
	dir string // ~/.claude by default
	cwd string // optional working directory hint for project selection
}

// NewClaudeCodeProvider constructs a Claude Code provider.
// cwd optionally narrows the search to a specific project directory.
func NewClaudeCodeProvider(dir, cwd string) PromptProvider {
	return &claudeCodeProvider{dir: dir, cwd: cwd}
}

func (p *claudeCodeProvider) defaultDir() string {
	if p.dir != "" {
		return p.dir
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".claude")
	}
	return ""
}

// Available reports whether Claude Code session data exists.
func (p *claudeCodeProvider) Available() bool {
	dir := p.defaultDir()
	if dir == "" {
		return false
	}
	projects := filepath.Join(dir, "projects")
	if info, err := os.Stat(projects); err == nil && info.IsDir() {
		// Look for any *.jsonl beneath
		var found bool
		_ = filepath.Walk(projects, func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() {
				return nil
			}
			if strings.HasSuffix(path, ".jsonl") {
				found = true
				return filepath.SkipAll
			}
			return nil
		})
		return found
	}
	return false
}

// LastModified returns the most recent mtime of any session file.
func (p *claudeCodeProvider) LastModified() (time.Time, bool) {
	dir := p.defaultDir()
	if dir == "" {
		return time.Time{}, false
	}
	projects := filepath.Join(dir, "projects")
	var latest time.Time
	_ = filepath.Walk(projects, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".jsonl") {
			if info.ModTime().After(latest) {
				latest = info.ModTime()
			}
		}
		return nil
	})
	return latest, !latest.IsZero()
}

func (p *claudeCodeProvider) History() ([]types.Prompt, error) {
	dir := p.defaultDir()
	if dir == "" {
		return nil, fmt.Errorf("claude code: cannot determine home directory")
	}

	projectsDir := filepath.Join(dir, "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil, fmt.Errorf("claude code: failed to read %s: %w", projectsDir, err)
	}

	var prompts []types.Prompt
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		subDir := filepath.Join(projectsDir, e.Name())
		files, err := os.ReadDir(subDir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".jsonl") {
				continue
			}
			fp := filepath.Join(subDir, f.Name())
			if err := readClaudeSession(fp, &prompts); err != nil {
				continue // skip unreadable sessions
			}
		}
	}
	return prompts, nil
}

// claudeSessionLine models the JSON shape of a single Claude Code
// session record. `content` may be a string or array of blocks.
type claudeSessionLine struct {
	Type    string             `json:"type"`
	Message *claudeMessagePart `json:"message"`
}

type claudeMessagePart struct {
	Role    string          `json:"role"`
	Content claudeContentRaw `json:"content"`
}

// claudeContentRaw accepts either a plain string or an array of blocks.
type claudeContentRaw struct {
	Raw  any
	Text string
}

func (c *claudeContentRaw) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	switch {
	case strings.HasPrefix(trimmed, "["):
		// Array of content blocks.
		var blocks []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if err := json.Unmarshal(data, &blocks); err == nil {
			var sb strings.Builder
			for _, b := range blocks {
				if b.Type == "text" && b.Text != "" {
					if sb.Len() > 0 {
						sb.WriteString("\n")
					}
					sb.WriteString(b.Text)
				}
			}
			c.Text = sb.String()
			c.Raw = blocks
		}
		return nil
	case strings.HasPrefix(trimmed, "\""):
		// JSON-encoded string — unquote via stdlib.
		var s string
		if err := json.Unmarshal(data, &s); err == nil {
			c.Text = s
			c.Raw = s
		}
		return nil
	default:
		// Plain unquoted text or unknown shape — store as-is.
		c.Text = trimmed
	}
	return nil
}

func readClaudeSession(path string, out *[]types.Prompt) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var rec claudeSessionLine
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}
		if rec.Message == nil {
			continue
		}
		content := strings.TrimSpace(rec.Message.Content.Text)
		if content == "" {
			continue
		}
		*out = append(*out, types.Prompt{
			Role:    rec.Message.Role,
			Content: truncateForContext(content, 4000),
		})
	}
	return scanner.Err()
}
