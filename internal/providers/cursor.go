package providers

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/RiddhiKatarki/ctx/pkg/types"
)

// cursorProvider detects a Cursor installation.
// NOTE: Cursor stores chat history in a SQLite database (state.vscdb)
// and/or workspaceStorage JSON files. Without a SQLite driver, this
// provider is currently detection-only. It is wired through the
// auto-detection registry so it shows up alongside other providers,
// and it returns an empty history to keep callers' flow unchanged.
//
// Future work: integrate a pure-Go SQLite driver (modernc.org/sqlite)
// to read aiService.generated/aiService.requests tables.
type cursorProvider struct {
	dir string
}

// NewCursorProvider constructs a Cursor provider that probes
// platform-appropriate install paths when dir is empty.
func NewCursorProvider(dir string) PromptProvider {
	return &cursorProvider{dir: dir}
}

func (p *cursorProvider) defaultDirs() []string {
	if p.dir != "" {
		return []string{p.dir}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	var paths []string
	switch runtime.GOOS {
	case "linux":
		paths = append(paths,
			filepath.Join(home, ".config", "Cursor"),
			filepath.Join(home, ".cursor"),
		)
		xdg := os.Getenv("XDG_CONFIG_HOME")
		if xdg != "" {
			paths = append(paths, filepath.Join(xdg, "Cursor"))
		}
	case "darwin":
		paths = append(paths,
			filepath.Join(home, "Library", "Application Support", "Cursor"),
		)
	case "windows":
		paths = append(paths,
			filepath.Join(os.Getenv("APPDATA"), "Cursor"),
		)
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			paths = append(paths, filepath.Join(local, "Cursor"))
		}
	}
	return paths
}

// Available reports whether a real Cursor install directory exists.
// Detection requires a known subdirectory (User/globalStorage) so
// we don't false-positive on users with custom override paths that
// happen to be empty temp dirs.
func (p *cursorProvider) Available() bool {
	for _, d := range p.defaultDirs() {
		if _, err := os.Stat(filepath.Join(d, "User")); err == nil {
			return true
		}
	}
	return false
}

// LastModified returns the mtime of the install directory as
// best-effort signal. Cursor chat record mtimes aren't accessible
// without a SQLite reader.
func (p *cursorProvider) LastModified() (time.Time, bool) {
	var latest time.Time
	for _, d := range p.defaultDirs() {
		if info, err := os.Stat(d); err == nil {
			if info.ModTime().After(latest) {
				latest = info.ModTime()
			}
		}
	}
	return latest, !latest.IsZero()
}

// History returns an empty prompt list. Cursor chat data lives
// inside SQLite; extraction requires a SQLite driver that is
// not yet wired. Use the Available() / LastModified() methods to
// verify an install exists.
func (p *cursorProvider) History() ([]types.Prompt, error) {
	// Look up at least one install to surface a helpful error.
	for _, d := range p.defaultDirs() {
		if info, err := os.Stat(d); err == nil && info.IsDir() {
			_ = info
			return nil, &cursorLimitedError{path: d}
		}
	}
	return nil, fmt.Errorf("cursor: no installation detected at %s", formatDirs(p.defaultDirs()))
}

// cursorLimitedError is returned when Cursor is detected but
// chat data cannot yet be extracted (no SQLite driver).
type cursorLimitedError struct {
	path string
}

func (e *cursorLimitedError) Error() string {
	return fmt.Sprintf(
		"cursor: install detected at %s, but chat extraction requires a SQLite driver (planned for a follow-up release)",
		e.path,
	)
}

func formatDirs(dirs []string) string {
	if len(dirs) == 0 {
		return "(no candidate paths)"
	}
	return strings.Join(dirs, ", ")
}
