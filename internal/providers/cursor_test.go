package providers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// TestCursorProvider_History_ModernSchema exercises the cursorDiskKV
// layout used by modern Cursor (3.x). Builds a synthetic state.vscdb
// with one composer holding two bubbles (user + assistant).
func TestCursorProvider_History_ModernSchema(t *testing.T) {
	dir := t.TempDir()
	globalDir := filepath.Join(dir, "User", "globalStorage")
	if err := os.MkdirAll(globalDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	dbPath := filepath.Join(globalDir, "state.vscdb")

	composerID := "composer-1"
	bubbles := []map[string]any{
		{"bubbleId": "b1", "type": 1, "text": "How do I parse JSON in Go?", "createdAt": 1750000000000},
		{"bubbleId": "b2", "type": 2, "text": "Use encoding/json with json.Unmarshal.", "createdAt": 1750000001000},
	}

	if err := buildModernDB(t, dbPath, composerID, "Test chat", bubbles); err != nil {
		t.Fatalf("buildModernDB: %v", err)
	}

	p := NewCursorProvider(dir, "")
	prompts, err := p.History()
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(prompts) != 2 {
		t.Fatalf("expected 2 prompts, got %d (%+v)", len(prompts), prompts)
	}
	if prompts[0].Role != "user" || prompts[0].Content != "How do I parse JSON in Go?" {
		t.Errorf("unexpected first prompt: %+v", prompts[0])
	}
	if prompts[1].Role != "assistant" || prompts[1].Content != "Use encoding/json with json.Unmarshal." {
		t.Errorf("unexpected second prompt: %+v", prompts[1])
	}
}

// TestCursorProvider_History_ModernSchema_SkipsUnknownBubbleTypes
// verifies the provider filters out anything that isn't type 1 or 2.
func TestCursorProvider_History_ModernSchema_SkipsUnknownBubbleTypes(t *testing.T) {
	dir := t.TempDir()
	globalDir := filepath.Join(dir, "User", "globalStorage")
	if err := os.MkdirAll(globalDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	dbPath := filepath.Join(globalDir, "state.vscdb")

	bubbles := []map[string]any{
		{"bubbleId": "b1", "type": 1, "text": "hello"},
		{"bubbleId": "b2", "type": 99, "text": "should be skipped"},
		{"bubbleId": "b3", "type": 2, "text": "hi back"},
		// Empty-text user bubble → skipped.
		{"bubbleId": "b4", "type": 1, "text": ""},
	}
	if err := buildModernDB(t, dbPath, "c1", "Test", bubbles); err != nil {
		t.Fatalf("buildModernDB: %v", err)
	}

	p := NewCursorProvider(dir, "")
	prompts, err := p.History()
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(prompts) != 2 {
		t.Fatalf("expected 2 prompts (filtered), got %d (%+v)", len(prompts), prompts)
	}
}

// TestCursorProvider_History_ModernSchema_RichTextFallback verifies
// that bubbles without `text` fall back to `richText` and `initText`.
func TestCursorProvider_History_ModernSchema_RichTextFallback(t *testing.T) {
	dir := t.TempDir()
	globalDir := filepath.Join(dir, "User", "globalStorage")
	if err := os.MkdirAll(globalDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	dbPath := filepath.Join(globalDir, "state.vscdb")

	bubbles := []map[string]any{
		{"bubbleId": "b1", "type": 1, "richText": "from rich text"},
		{"bubbleId": "b2", "type": 2, "initText": `{"root":{"children":[{"children":[{"text":"from lexical"}]}]}}`},
	}
	if err := buildModernDB(t, dbPath, "c1", "Test", bubbles); err != nil {
		t.Fatalf("buildModernDB: %v", err)
	}

	p := NewCursorProvider(dir, "")
	prompts, err := p.History()
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(prompts) != 2 {
		t.Fatalf("expected 2 prompts, got %d", len(prompts))
	}
	if prompts[0].Content != "from rich text" {
		t.Errorf("expected rich-text fallback, got %q", prompts[0].Content)
	}
	if prompts[1].Content != "from lexical" {
		t.Errorf("expected lexical init-text fallback, got %q", prompts[1].Content)
	}
}

// TestCursorProvider_History_LegacySchema exercises the older
// per-workspace ItemTable chat layout used by Cursor <= 0.40.
func TestCursorProvider_History_LegacySchema(t *testing.T) {
	dir := t.TempDir()
	wsDir := filepath.Join(dir, "User", "workspaceStorage", "abc123")
	if err := os.MkdirAll(wsDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	dbPath := filepath.Join(wsDir, "state.vscdb")

	chatData := map[string]any{
		"tabs": []map[string]any{
			{
				"bubbles": []map[string]any{
					{"type": "user", "text": "What is context.Context?"},
					{"type": "ai", "text": "It carries deadlines and cancellation."},
				},
			},
		},
	}
	raw, _ := json.Marshal(chatData)
	if err := buildLegacyDB(t, dbPath, map[string][]byte{
		"workbench.panel.aichat.view.aichat.chatdata": raw,
	}); err != nil {
		t.Fatalf("buildLegacyDB: %v", err)
	}

	p := NewCursorProvider(dir, "")
	prompts, err := p.History()
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(prompts) != 2 {
		t.Fatalf("expected 2 legacy prompts, got %d", len(prompts))
	}
	if prompts[0].Role != "user" || prompts[0].Content != "What is context.Context?" {
		t.Errorf("unexpected legacy first prompt: %+v", prompts[0])
	}
	if prompts[1].Role != "assistant" {
		t.Errorf("expected assistant role for legacy ai bubble, got %s", prompts[1].Role)
	}
}

// TestCursorProvider_History_CwdFilter verifies the project-scope
// filter: only the composer bound to a matching fsPath is returned.
func TestCursorProvider_History_CwdFilter(t *testing.T) {
	dir := t.TempDir()
	globalDir := filepath.Join(dir, "User", "globalStorage")
	if err := os.MkdirAll(globalDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	dbPath := filepath.Join(globalDir, "state.vscdb")

	// Two composers: one for /home/me/projA (match), one for /home/me/projB (skip).
	matchCwd := "/home/me/projA"
	mismatchCwd := "/home/me/projB"

	matchBubbles := []map[string]any{{"bubbleId": "a1", "type": 1, "text": "from projA"}}
	skipBubbles := []map[string]any{{"bubbleId": "b1", "type": 1, "text": "from projB"}}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	for _, stmt := range []string{
		`CREATE TABLE cursorDiskKV (key TEXT PRIMARY KEY, value BLOB)`,
		`CREATE TABLE ItemTable (key TEXT PRIMARY KEY, value BLOB)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}

	put := func(table, key string, val any) {
		raw, _ := json.Marshal(val)
		q := fmt.Sprintf(`INSERT INTO %s (key, value) VALUES (?, ?)`, quoteIdent(table))
		if _, err := db.Exec(q, key, raw); err != nil {
			t.Fatalf("insert %s/%s: %v", table, key, err)
		}
	}

	put("cursorDiskKV", "composerData:cA", map[string]any{"composerId": "cA", "createdAt": 1750000000000})
	put("cursorDiskKV", "composerData:cB", map[string]any{"composerId": "cB", "createdAt": 1750000001000})
	put("cursorDiskKV", "bubbleId:cA:a1", matchBubbles[0])
	put("cursorDiskKV", "bubbleId:cB:b1", skipBubbles[0])

	// composer.composerHeaders — order matters for "newest first".
	put("ItemTable", "composer.composerHeaders", map[string]any{
		"allComposers": []map[string]any{
			{"composerId": "cB", "name": "projB chat", "lastUpdatedAt": 1750000001000},
			{"composerId": "cA", "name": "projA chat", "lastUpdatedAt": 1750000000000},
		},
	})
	put("ItemTable", "glass.localAgentProjects.v1", []map[string]any{
		{"id": "pA", "name": "projA", "workspace": map[string]any{"uri": map[string]any{"fsPath": matchCwd}}},
		{"id": "pB", "name": "projB", "workspace": map[string]any{"uri": map[string]any{"fsPath": mismatchCwd}}},
	})
	put("ItemTable", "glass.localAgentProjectMembership.v1", map[string]any{
		"cA": "pA",
		"cB": "pB",
	})
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// No cwd filter → both composers returned.
	pAll := NewCursorProvider(dir, "")
	all, err := pAll.History()
	if err != nil {
		t.Fatalf("History (no cwd): %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 prompts without cwd filter, got %d", len(all))
	}

	// Cwd = matchCwd → only the projA bubble.
	pF := NewCursorProvider(dir, matchCwd)
	filtered, err := pF.History()
	if err != nil {
		t.Fatalf("History (filtered): %v", err)
	}
	if len(filtered) != 1 {
		t.Fatalf("expected 1 prompt with cwd filter, got %d (%+v)", len(filtered), filtered)
	}
	if filtered[0].Content != "from projA" {
		t.Errorf("expected projA content, got %q", filtered[0].Content)
	}
}

// TestCursorProvider_LastModified_DB beats User/ dir mtime.
func TestCursorProvider_LastModified_DB(t *testing.T) {
	dir := t.TempDir()
	globalDir := filepath.Join(dir, "User", "globalStorage")
	if err := os.MkdirAll(globalDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	dbPath := filepath.Join(globalDir, "state.vscdb")
	if err := buildModernDB(t, dbPath, "c1", "x", []map[string]any{
		{"bubbleId": "b1", "type": 1, "text": "hi"},
	}); err != nil {
		t.Fatalf("buildModernDB: %v", err)
	}

	p := NewCursorProvider(dir, "")
	ts, ok := p.(AvailChecker).LastModified()
	if !ok {
		t.Fatal("expected ok=true")
	}
	if ts.IsZero() {
		t.Error("expected non-zero mtime from state.vscdb")
	}
}

// ---- Fixture builders ----------------------------------------------------

// buildModernDB creates a state.vscdb with the modern cursorDiskKV
// schema and a single composer holding the given bubbles.
func buildModernDB(t *testing.T, dbPath, composerID, name string, bubbles []map[string]any) error {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer db.Close()

	for _, stmt := range []string{
		`CREATE TABLE cursorDiskKV (key TEXT PRIMARY KEY, value BLOB)`,
		`CREATE TABLE ItemTable (key TEXT PRIMARY KEY, value BLOB)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt, err)
		}
	}

	// composerData:<id>
	cd, _ := json.Marshal(map[string]any{
		"composerId":                 composerID,
		"name":                       name,
		"createdAt":                  1750000000000,
		"lastUpdatedAt":              1750000002000,
		"fullConversationHeadersOnly": bubbleHeaders(bubbles),
	})
	if _, err := db.Exec(`INSERT INTO cursorDiskKV (key, value) VALUES (?, ?)`, "composerData:"+composerID, cd); err != nil {
		return fmt.Errorf("insert composerData: %w", err)
	}

	// ItemTable composer.composerHeaders (so the provider exercises
	// the structured-loading path rather than the discovery fallback).
	hdr, _ := json.Marshal(map[string]any{
		"allComposers": []map[string]any{
			{"composerId": composerID, "name": name, "lastUpdatedAt": 1750000002000},
		},
	})
	if _, err := db.Exec(`INSERT INTO ItemTable (key, value) VALUES (?, ?)`, "composer.composerHeaders", hdr); err != nil {
		return fmt.Errorf("insert composerHeaders: %w", err)
	}

	// One row per bubble.
	for _, b := range bubbles {
		bid, _ := b["bubbleId"].(string)
		if bid == "" {
			return fmt.Errorf("bubble missing bubbleId: %+v", b)
		}
		raw, _ := json.Marshal(b)
		key := "bubbleId:" + composerID + ":" + bid
		if _, err := db.Exec(`INSERT INTO cursorDiskKV (key, value) VALUES (?, ?)`, key, raw); err != nil {
			return fmt.Errorf("insert %s: %w", key, err)
		}
	}
	return nil
}

func bubbleHeaders(bubbles []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(bubbles))
	for _, b := range bubbles {
		bid, _ := b["bubbleId"].(string)
		t, _ := b["type"].(int)
		out = append(out, map[string]any{"bubbleId": bid, "type": t})
	}
	return out
}

// buildLegacyDB creates a state.vscdb with the ItemTable schema and
// inserts the given (key, value) rows.
func buildLegacyDB(t *testing.T, dbPath string, rows map[string][]byte) error {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE ItemTable (key TEXT PRIMARY KEY, value BLOB)`); err != nil {
		return fmt.Errorf("create ItemTable: %w", err)
	}
	for k, v := range rows {
		if _, err := db.Exec(`INSERT INTO ItemTable (key, value) VALUES (?, ?)`, k, v); err != nil {
			return fmt.Errorf("insert %s: %w", k, err)
		}
	}
	return nil
}
