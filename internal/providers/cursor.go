package providers

import (
	database_sql "database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/RiddhiKatarki/ctx/pkg/types"

	// Pure-Go SQLite driver (no cgo) — registers as "sqlite" with database/sql.
	_ "modernc.org/sqlite"
)

// cursorProvider reads Cursor's chat history from the SQLite databases
// (state.vscdb) that Cursor writes under User/globalStorage and
// User/workspaceStorage/<hash>/.
//
// Two schemas are supported:
//
//  1. Modern (Cursor 3.x): the global state.vscdb has a `cursorDiskKV`
//     table with `composerData:<id>` and `bubbleId:<cid>:<bid>` rows
//     holding JSON chat metadata and per-message turns.
//
//  2. Legacy (Cursor <= ~0.40): per-workspace state.vscdb has an
//     `ItemTable` row keyed `workbench.panel.aichat.view.aichat.chatdata`
//     holding the entire chat as a single JSON blob.
//
// The provider tries modern first, falls back to legacy. When a
// working directory is supplied, chats are filtered to that project
// via the global ItemTable's project membership tables; otherwise all
// chats are returned (most recent first).
type cursorProvider struct {
	dir string // Cursor install root (parent of User/)
	cwd string // optional working directory for project-scoped filtering
}

// NewCursorProvider constructs a Cursor provider. dir is the Cursor
// install root (e.g. ~/.config/Cursor); when empty, platform-appropriate
// paths are probed. cwd optionally scopes results to a project.
func NewCursorProvider(dir, cwd string) PromptProvider {
	return &cursorProvider{dir: dir, cwd: cwd}
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

// userDir returns the first existing User/ subdirectory across the
// probed install roots, or "" if none.
func (p *cursorProvider) userDir() string {
	for _, d := range p.defaultDirs() {
		userDir := filepath.Join(d, "User")
		if info, err := os.Stat(userDir); err == nil && info.IsDir() {
			return userDir
		}
	}
	return ""
}

// globalDBPath returns the path to User/globalStorage/state.vscdb
// inside the first install that has it, or "" if none.
func (p *cursorProvider) globalDBPath() string {
	userDir := p.userDir()
	if userDir == "" {
		return ""
	}
	return filepath.Join(userDir, "globalStorage", "state.vscdb")
}

// workspaceDBPaths returns state.vscdb files under User/workspaceStorage/*/.
func (p *cursorProvider) workspaceDBPaths() []string {
	userDir := p.userDir()
	if userDir == "" {
		return nil
	}
	wsRoot := filepath.Join(userDir, "workspaceStorage")
	entries, err := os.ReadDir(wsRoot)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dbPath := filepath.Join(wsRoot, e.Name(), "state.vscdb")
		if _, err := os.Stat(dbPath); err == nil {
			out = append(out, dbPath)
		}
	}
	return out
}

// Available reports whether a Cursor install with a User/ directory
// exists. (Database files may or may not be present yet.)
func (p *cursorProvider) Available() bool {
	return p.userDir() != ""
}

// LastModified returns the mtime of the most recently modified
// state.vscdb file (global + workspace). Falls back to the User/
// directory mtime when no DB exists yet.
func (p *cursorProvider) LastModified() (time.Time, bool) {
	var latest time.Time
	candidates := []string{p.globalDBPath()}
	candidates = append(candidates, p.workspaceDBPaths()...)
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if info, err := os.Stat(c); err == nil {
			if info.ModTime().After(latest) {
				latest = info.ModTime()
			}
		}
	}
	if !latest.IsZero() {
		return latest, true
	}
	// Fallback: User/ directory mtime.
	if userDir := p.userDir(); userDir != "" {
		if info, err := os.Stat(userDir); err == nil {
			return info.ModTime(), true
		}
	}
	return time.Time{}, false
}

// History reads Cursor chat history. Modern schema (cursorDiskKV) is
// tried first; legacy schema (ItemTable) is the fallback. When a cwd
// is configured, modern results are filtered to that project.
//
// A Cursor install with no chat history returns an empty list without
// error; only unreadable/corrupt databases error.
func (p *cursorProvider) History() ([]types.Prompt, error) {
	if p.userDir() == "" {
		return nil, fmt.Errorf("cursor: no installation detected at %s", formatDirs(p.defaultDirs()))
	}

	globalDB := p.globalDBPath()
	globalExists := globalDB != ""
	if globalExists {
		if _, err := os.Stat(globalDB); err == nil {
			globalExists = true
		} else {
			globalExists = false
		}
	}

	// Modern path first (if global DB exists).
	if globalExists {
		prompts, err := p.readModern(globalDB)
		if err == nil && len(prompts) > 0 {
			return prompts, nil
		}
		if err != nil && p.cwd == "" && len(p.workspaceDBPaths()) == 0 {
			// No legacy fallback possible and modern errored — surface it.
			return nil, fmt.Errorf("cursor: read %s: %w", globalDB, err)
		}
	}

	// Legacy fallback: scan per-workspace DBs.
	legacy, legacyErr := p.readLegacy()
	if legacyErr == nil && len(legacy) > 0 {
		return legacy, nil
	}

	// Nothing extracted.
	if globalExists && legacyErr != nil {
		// Both paths tried, both empty. That's a legitimate empty state.
		return []types.Prompt{}, nil
	}
	if !globalExists && legacyErr != nil {
		return []types.Prompt{}, nil
	}
	return []types.Prompt{}, nil
}

// ---- Modern schema (cursorDiskKV) ---------------------------------------

func (p *cursorProvider) readModern(dbPath string) ([]types.Prompt, error) {
	db, err := openReadOnly(dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	// Confirm cursorDiskKV exists.
	hasTable, err := tableExists(db, "cursorDiskKV")
	if err != nil {
		return nil, err
	}
	if !hasTable {
		return nil, nil
	}

	// Load composer headers (id + lastUpdatedAt) to order chats newest-first.
	composers, err := p.loadComposers(db)
	if err != nil {
		return nil, err
	}

	// Filter by cwd when supplied: resolve composer IDs belonging to
	// the project whose workspace.uri.fsPath matches cwd.
	if p.cwd != "" {
		filtered, err := p.filterByCwd(db, composers)
		if err == nil && len(filtered) > 0 {
			composers = filtered
		}
		// On error, fall through with unfiltered set.
	}

	// Sort newest first.
	sort.SliceStable(composers, func(i, j int) bool {
		return composers[i].updatedAt.After(composers[j].updatedAt)
	})

	var all []types.Prompt
	for _, c := range composers {
		bubbles, err := loadComposerBubbles(db, c.id)
		if err != nil {
			continue
		}
		all = append(all, bubbles...)
	}
	return all, nil
}

type composerRef struct {
	id        string
	updatedAt time.Time
}

// loadComposers reads ItemTable['composer.composerHeaders'] which
// lists every chat with its lastUpdatedAt timestamp.
func (p *cursorProvider) loadComposers(db *database_sql.DB) ([]composerRef, error) {
	hasItem, err := tableExists(db, "ItemTable")
	if err != nil {
		return nil, err
	}
	if !hasItem {
		return discoverComposersFromDiskKV(db)
	}

	var raw []byte
	row := db.QueryRow(`SELECT value FROM ItemTable WHERE key = 'composer.composerHeaders' LIMIT 1`)
	err = row.Scan(&raw)
	if err == database_sql.ErrNoRows {
		return discoverComposersFromDiskKV(db)
	}
	if err != nil {
		return nil, err
	}

	var hdr struct {
		AllComposers []struct {
			ComposerID   string `json:"composerId"`
			Name         string `json:"name"`
			LastUpdatedAt int64 `json:"lastUpdatedAt"`
		} `json:"allComposers"`
	}
	if err := json.Unmarshal(raw, &hdr); err != nil {
		return discoverComposersFromDiskKV(db)
	}
	out := make([]composerRef, 0, len(hdr.AllComposers))
	for _, c := range hdr.AllComposers {
		out = append(out, composerRef{
			id:        c.ComposerID,
			updatedAt: unixMs(c.LastUpdatedAt),
		})
	}
	if len(out) == 0 {
		return discoverComposersFromDiskKV(db)
	}
	return out, nil
}

// discoverComposersFromDiskKV is the fallback when composerHeaders
// is missing/unparseable: scan cursorDiskKV for `composerData:%` rows.
func discoverComposersFromDiskKV(db *database_sql.DB) ([]composerRef, error) {
	rows, err := db.Query(`SELECT key, value FROM cursorDiskKV WHERE key LIKE 'composerData:%'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []composerRef
	for rows.Next() {
		var key, value []byte
		if err := rows.Scan(&key, &value); err != nil {
			continue
		}
		var cd struct {
			ComposerID    string `json:"composerId"`
			CreatedAt     int64  `json:"createdAt"`
			LastUpdatedAt int64  `json:"lastUpdatedAt"`
		}
		_ = json.Unmarshal(value, &cd)
		id := cd.ComposerID
		if id == "" {
			id = strings.TrimPrefix(string(key), "composerData:")
		}
		ts := unixMs(cd.LastUpdatedAt)
		if ts.IsZero() {
			ts = unixMs(cd.CreatedAt)
		}
		out = append(out, composerRef{id: id, updatedAt: ts})
	}
	return out, rows.Err()
}

// filterByCwd returns only composers that belong to a project whose
// workspace path matches p.cwd (best-effort prefix match on absolute paths).
func (p *cursorProvider) filterByCwd(db *database_sql.DB, composers []composerRef) ([]composerRef, error) {
	if p.cwd == "" || len(composers) == 0 {
		return composers, nil
	}

	// Load project membership (composerId -> projectId).
	membership, err := kvJSON(db, "ItemTable", "glass.localAgentProjectMembership.v1")
	if err != nil {
		return composers, nil
	}
	projByComposer := map[string]string{}
	if m, ok := membership.(map[string]any); ok {
		for k, v := range m {
			if s, ok := v.(string); ok {
				projByComposer[k] = s
			}
		}
	}

	// Load projects list (projectId -> fsPath).
	projects, err := kvJSON(db, "ItemTable", "glass.localAgentProjects.v1")
	if err != nil {
		return composers, nil
	}
	pathByProject := map[string]string{}
	if arr, ok := projects.([]any); ok {
		for _, item := range arr {
			obj, ok := item.(map[string]any)
			if !ok {
				continue
			}
			id, _ := obj["id"].(string)
			ws, _ := obj["workspace"].(map[string]any)
			uri, _ := ws["uri"].(map[string]any)
			fsPath, _ := uri["fsPath"].(string)
			if id != "" && fsPath != "" {
				pathByProject[id] = fsPath
			}
		}
	}

	if len(pathByProject) == 0 {
		return composers, nil
	}

	cwdAbs, err := filepath.Abs(p.cwd)
	if err != nil {
		cwdAbs = p.cwd
	}
	var out []composerRef
	for _, c := range composers {
		projID := projByComposer[c.id]
		fsPath := pathByProject[projID]
		if fsPath == "" {
			continue
		}
		if samePathOrUnder(fsPath, cwdAbs) {
			out = append(out, c)
		}
	}
	return out, nil
}

// loadComposerBubbles reads every `bubbleId:<cid>:*` row in order and
// returns typed prompts.
func loadComposerBubbles(db *database_sql.DB, composerID string) ([]types.Prompt, error) {
	prefix := "bubbleId:" + composerID + ":%"
	rows, err := db.Query(`SELECT value FROM cursorDiskKV WHERE key LIKE ? ORDER BY key`, prefix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []types.Prompt
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			continue
		}
		var b struct {
			BubbleID string `json:"bubbleId"`
			Type     int    `json:"type"` // 1=user, 2=assistant
			Text     string `json:"text"`
			RichText string `json:"richText"`
			InitText string `json:"initText"`
			Delegate struct {
				A string `json:"a"` // legacy user wrapper
			} `json:"delegate"`
			CreatedAt int64 `json:"createdAt"`
		}
		if err := json.Unmarshal(raw, &b); err != nil {
			continue
		}
		text := bubbleText(b.Text, b.RichText, b.InitText, b.Delegate.A)
		role := bubbleRole(b.Type, text)
		if role == "" || strings.TrimSpace(text) == "" {
			continue
		}
		out = append(out, types.Prompt{Role: role, Content: truncateForContext(text, 4000)})
	}
	return out, rows.Err()
}

// bubbleText extracts the human-readable content of a bubble across
// the various Cursor storage conventions.
func bubbleText(primary, rich, init, legacy string) string {
	if strings.TrimSpace(primary) != "" {
		return primary
	}
	if strings.TrimSpace(rich) != "" {
		return rich
	}
	if strings.TrimSpace(init) != "" {
		// Lexical editor JSON sometimes wraps plain text; best-effort unwrap.
		if t := extractLexicalText(init); t != "" {
			return t
		}
		return init
	}
	return legacy
}

// extractLexicalText best-effort extracts text from a Lexical JSON
// node tree (root.children[].children[].text).
func extractLexicalText(s string) string {
	var doc struct {
		Root struct {
			Children []struct {
				Children []struct {
					Text string `json:"text"`
				} `json:"children"`
			} `json:"children"`
		} `json:"root"`
	}
	if err := json.Unmarshal([]byte(s), &doc); err != nil {
		return ""
	}
	var parts []string
	for _, c := range doc.Root.Children {
		for _, cc := range c.Children {
			if cc.Text != "" {
				parts = append(parts, cc.Text)
			}
		}
	}
	return strings.Join(parts, "\n")
}

// bubbleRole maps a Cursor bubble type to a prompt role. Empty result
// filters out unknown types (system/tool rows).
func bubbleRole(t int, text string) string {
	switch t {
	case 1:
		return "user"
	case 2:
		return "assistant"
	}
	return ""
}

// ---- Legacy schema (ItemTable workbench.panel.aichat.*) -----------------

func (p *cursorProvider) readLegacy() ([]types.Prompt, error) {
	for _, dbPath := range p.workspaceDBPaths() {
		db, err := openReadOnly(dbPath)
		if err != nil {
			continue
		}
		raw, err := kvBytes(db, "ItemTable", "workbench.panel.aichat.view.aichat.chatdata")
		_ = db.Close()
		if err != nil || len(raw) == 0 {
			continue
		}
		if prompts := parseLegacyChatData(raw); len(prompts) > 0 {
			return prompts, nil
		}
	}
	return nil, fmt.Errorf("cursor: no chat history found in workspace DBs")
}

// parseLegacyChatData extracts prompts from a legacy aichat.chatdata blob.
// Shape: {"tabs":[{bubbles:[{type:"user"|"ai", text, rawText, initText}]}]}
func parseLegacyChatData(raw []byte) []types.Prompt {
	var doc struct {
		Tabs []struct {
			Bubbles []struct {
				Type     string `json:"type"`
				Text     string `json:"text"`
				RawText  string `json:"rawText"`
				InitText string `json:"initText"`
			} `json:"bubbles"`
		} `json:"tabs"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil
	}
	var out []types.Prompt
	for _, tab := range doc.Tabs {
		for _, b := range tab.Bubbles {
			role := ""
			switch strings.ToLower(b.Type) {
			case "user":
				role = "user"
			case "ai", "assistant":
				role = "assistant"
			}
			if role == "" {
				continue
			}
			text := b.Text
			if strings.TrimSpace(text) == "" {
				text = b.RawText
			}
			if strings.TrimSpace(text) == "" {
				text = b.InitText
			}
			if strings.TrimSpace(text) == "" {
				continue
			}
			out = append(out, types.Prompt{Role: role, Content: truncateForContext(text, 4000)})
		}
	}
	return out
}

// ---- DB helpers ----------------------------------------------------------

// openReadOnly opens a SQLite file in immutable, read-only URI mode.
// Cursor holds an exclusive lock while running; this avoids WAL tears.
func openReadOnly(path string) (*database_sql.DB, error) {
	uri := fmt.Sprintf("file:%s?mode=ro&immutable=1", path)
	db, err := database_sql.Open("sqlite", uri)
	if err != nil {
		return nil, err
	}
	// Confirm we can actually reach the file.
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func tableExists(db *database_sql.DB, name string) (bool, error) {
	var cnt int
	err := db.QueryRow(
		`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?`,
		name,
	).Scan(&cnt)
	if err != nil {
		return false, err
	}
	return cnt > 0, nil
}

// kvBytes returns the raw value bytes for (table, key), or sql.ErrNoRows.
func kvBytes(db *database_sql.DB, table, key string) ([]byte, error) {
	q := fmt.Sprintf(`SELECT value FROM %s WHERE key = ? LIMIT 1`, quoteIdent(table))
	var raw []byte
	err := db.QueryRow(q, key).Scan(&raw)
	return raw, err
}

// kvJSON returns the JSON-decoded value for (table, key).
func kvJSON(db *database_sql.DB, table, key string) (any, error) {
	raw, err := kvBytes(db, table, key)
	if err != nil {
		return nil, err
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	return v, nil
}

// quoteIdent wraps a SQLite identifier in double quotes. Table names
// here are static literals, so this is purely defensive.
func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// unixMs converts a unix-millis int to a time.Time. Zero/negative → zero time.
func unixMs(ms int64) time.Time {
	if ms <= 0 {
		return time.Time{}
	}
	return time.Unix(0, ms*int64(time.Millisecond))
}

// samePathOrUnder reports whether a is equal to or an ancestor of b
// (case-insensitive on Windows / case-sensitive elsewhere).
func samePathOrUnder(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if runtime.GOOS == "windows" {
		a = strings.ToLower(a)
		b = strings.ToLower(b)
	}
	if a == b {
		return true
	}
	return strings.HasPrefix(b, a+string(filepath.Separator))
}

func formatDirs(dirs []string) string {
	if len(dirs) == 0 {
		return "(no candidate paths)"
	}
	return strings.Join(dirs, ", ")
}
