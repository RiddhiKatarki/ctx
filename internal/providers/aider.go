package providers

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/RiddhiKatarki/ctx/pkg/types"
)

// aiderProvider reads prompt history from Aider's chat log files.
// Two formats are supported:
//   1. ~/.aider.chat.history (older) — "USER: ..." / "ASSISTANT: ..." lines
//   2. ~/.aider.chat.history.md or ./.aider.chat.history.md (newer) —
//      "#### {role}" headings followed by free-form text.
//
// Both formats alternate user/assistant turns. The provider returns
// the conversation as []types.Prompt.
type aiderProvider struct {
	dir string // ~/.aider by default
	cwd string // optional project directory for .aider.chat.history.md lookup
}

// NewAiderProvider constructs an Aider prompt provider.
func NewAiderProvider(dir, cwd string) PromptProvider {
	return &aiderProvider{dir: dir, cwd: cwd}
}

func (p *aiderProvider) globalPath() string {
	if p.dir != "" {
		return filepath.Join(p.dir, ".aider.chat.history")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".aider.chat.history")
	}
	return ""
}

func (p *aiderProvider) globalMDPath() string {
	if p.dir != "" {
		return filepath.Join(p.dir, ".aider.chat.history.md")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".aider.chat.history.md")
	}
	return ""
}

func (p *aiderProvider) localMDPath() string {
	if p.cwd == "" {
		return ""
	}
	return filepath.Join(p.cwd, ".aider.chat.history.md")
}

// Available reports whether any known Aider chat log exists.
func (p *aiderProvider) Available() bool {
	for _, p := range []string{p.globalPath(), p.globalMDPath(), p.localMDPath()} {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

// LastModified returns the mtime of the most recently touched known file.
func (p *aiderProvider) LastModified() (time.Time, bool) {
	var latest time.Time
	for _, p := range []string{p.globalPath(), p.globalMDPath(), p.localMDPath()} {
		if p == "" {
			continue
		}
		fi, err := os.Stat(p)
		if err != nil {
			continue
		}
		if fi.ModTime().After(latest) {
			latest = fi.ModTime()
		}
	}
	return latest, !latest.IsZero()
}

func (p *aiderProvider) History() ([]types.Prompt, error) {
	// Try project-local first, then user-global files.
	candidates := []string{p.localMDPath(), p.globalMDPath(), p.globalPath()}
	var found string
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if _, err := os.Stat(c); err == nil {
			found = c
			break
		}
	}
	if found == "" {
		return nil, fmt.Errorf("aider: no chat history found (looked in cwd, ~/.aider)")
	}

	content, err := os.ReadFile(found)
	if err != nil {
		return nil, fmt.Errorf("aider: failed to read %s: %w", found, err)
	}

	if strings.HasSuffix(found, ".md") {
		return parseAiderMarkdown(string(content)), nil
	}
	return parseAiderPlain(string(content)), nil
}

func parseAiderPlain(content string) []types.Prompt {
	var prompts []types.Prompt
	scanner := bufio.NewScanner(strings.NewReader(content))
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)

	var currentRole, currentText string
	flush := func() {
		if strings.TrimSpace(currentText) != "" {
			prompts = append(prompts, types.Prompt{
				Role:    strings.ToLower(currentRole),
				Content: truncateForContext(strings.TrimSpace(currentText), 4000),
			})
		}
		currentRole, currentText = "", ""
	}

	for scanner.Scan() {
		line := scanner.Text()
		upper := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case strings.HasPrefix(upper, "USER:"):
			flush()
			currentRole = "user"
			currentText = strings.TrimSpace(line[len("USER:"):])
		case strings.HasPrefix(upper, "ASSISTANT:"):
			flush()
			currentRole = "assistant"
			currentText = strings.TrimSpace(line[len("ASSISTANT:"):])
		case strings.HasPrefix(upper, "SYSTEM:"):
			flush()
			currentRole = "system"
			currentText = strings.TrimSpace(line[len("SYSTEM:"):])
		default:
			if currentText != "" {
				currentText += "\n" + line
			}
		}
	}
	flush()
	return prompts
}

func parseAiderMarkdown(content string) []types.Prompt {
	var prompts []types.Prompt
	scanner := bufio.NewScanner(strings.NewReader(content))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	var currentRole, currentText string
	flush := func() {
		if strings.TrimSpace(currentText) != "" {
			prompts = append(prompts, types.Prompt{
				Role:    currentRole,
				Content: truncateForContext(strings.TrimSpace(currentText), 4000),
			})
		}
		currentRole, currentText = "", ""
	}

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "####") {
			flush()
			rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "####"))
			rest = strings.TrimSpace(strings.TrimPrefix(rest, "user"))
			rest = strings.TrimSpace(strings.TrimPrefix(rest, "assistant"))
			rest = strings.TrimSpace(strings.TrimPrefix(rest, "system"))
			switch {
			case strings.HasPrefix(strings.ToLower(trimmed), "#### user"):
				currentRole = "user"
			case strings.HasPrefix(strings.ToLower(trimmed), "#### assistant"):
				currentRole = "assistant"
			case strings.HasPrefix(strings.ToLower(trimmed), "#### system"):
				currentRole = "system"
			default:
				currentRole = strings.ToLower(rest)
			}
			continue
		}
		if currentRole != "" {
			if currentText != "" {
				currentText += "\n" + line
			} else {
				currentText = line
			}
		}
	}
	flush()
	return prompts
}
