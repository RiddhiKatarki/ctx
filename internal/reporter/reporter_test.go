package reporter

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
	"testing"
)

func TestNew_Defaults_Human(t *testing.T) {
	r := New(ModeHuman, nil, nil)
	if _, ok := r.(*HumanReporter); !ok {
		t.Errorf("expected HumanReporter for ModeHuman, got %T", r)
	}
}

func TestNew_JSON(t *testing.T) {
	r := New(ModeJSON, nil, nil)
	if _, ok := r.(*JSONReporter); !ok {
		t.Errorf("expected JSONReporter for ModeJSON, got %T", r)
	}
}

func TestNew_Stream(t *testing.T) {
	r := New(ModeStream, nil, nil)
	if _, ok := r.(*StreamReporter); !ok {
		t.Errorf("expected StreamReporter for ModeStream, got %T", r)
	}
}

func TestNew_FallbackToHuman(t *testing.T) {
	r := New("bogus", nil, nil)
	if _, ok := r.(*HumanReporter); !ok {
		t.Errorf("expected HumanReporter for unknown mode, got %T", r)
	}
}

func TestHumanReporter_Event_IsSilentInHumanMode(t *testing.T) {
	var buf bytes.Buffer
	r := NewHumanReporter(&buf)

	r.Info("hello %s\n", "world")
	r.Event("git_metadata", map[string]any{"branch": "main"})

	out := buf.String()
	if !strings.Contains(out, "hello world") {
		t.Errorf("expected 'hello world' in output, got %q", out)
	}
	if strings.Contains(out, "git_metadata") {
		t.Errorf("expected Event() to be silent in human mode, got %q", out)
	}
}

func TestHumanReporter_Info_Only(t *testing.T) {
	var buf bytes.Buffer
	r := NewHumanReporter(&buf)

	r.Info("line1\n")
	r.Info("line2\n")

	out := buf.String()
	if !strings.Contains(out, "line1") || !strings.Contains(out, "line2") {
		t.Errorf("expected both lines in output, got %q", out)
	}
}

func TestHumanReporter_NilWriter_DefaultsToStderr(t *testing.T) {
	// Should not panic when constructed with nil writer.
	r := NewHumanReporter(nil)
	r.Info("ok\n") // writes to default os.Stderr; we just verify no panic
}

func TestJSONReporter_Done_EmitsResult(t *testing.T) {
	var buf bytes.Buffer
	r := NewJSONReporter(&buf)

	r.Event("git_metadata", map[string]any{"branch": "main"})
	r.Info("ignored line\n")

	result := map[string]any{"path": "project.ctx", "files": 5}
	if err := r.Done(result); err != nil {
		t.Fatalf("Done: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}

	if parsed["path"] != "project.ctx" {
		t.Errorf("expected path 'project.ctx', got %v", parsed["path"])
	}
	if parsed["files"].(float64) != 5 {
		t.Errorf("expected files=5, got %v", parsed["files"])
	}
}

func TestStreamReporter_Event_EmitsNDJSON(t *testing.T) {
	var buf bytes.Buffer
	r := NewStreamReporter(&buf)

	if err := r.Event("git_metadata", map[string]any{"branch": "main"}); err != nil {
		t.Fatalf("Event: %v", err)
	}
	if err := r.Event("prompt_collected", map[string]any{"count": 3}); err != nil {
		t.Fatalf("Event: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 NDJSON lines, got %d:\n%s", len(lines), buf.String())
	}

	var first map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("line 0 is not JSON: %v\nline: %s", err, lines[0])
	}
	if first["event"] != "git_metadata" {
		t.Errorf("expected event 'git_metadata', got %v", first["event"])
	}
	data := first["data"].(map[string]any)
	if data["branch"] != "main" {
		t.Errorf("expected branch 'main', got %v", data["branch"])
	}
}

func TestStreamReporter_Done_EmitsCompleteLine(t *testing.T) {
	var buf bytes.Buffer
	r := NewStreamReporter(&buf)

	result := map[string]any{"path": "out.ctx", "size": 2500}
	if err := r.Done(result); err != nil {
		t.Fatalf("Done: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	var line map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &line); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if line["event"] != "complete" {
		t.Errorf("expected event 'complete', got %v", line["event"])
	}
	d := line["data"].(map[string]any)
	if d["path"] != "out.ctx" {
		t.Errorf("expected path 'out.ctx', got %v", d["path"])
	}
}

func TestStreamReporter_Error(t *testing.T) {
	var buf bytes.Buffer
	r := NewStreamReporter(&buf)

	err := r.Error("error", &testErr{"disk full"})
	if err != nil {
		t.Fatalf("Error: %v", err)
	}

	var line map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &line); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if line["event"] != "error" {
		t.Errorf("expected event 'error', got %v", line["event"])
	}
	d := line["data"].(map[string]any)
	if d["message"] != "disk full" {
		t.Errorf("expected message 'disk full', got %v", d["message"])
	}
}

func TestJSONReporter_Info_DoesNothing(t *testing.T) {
	var buf bytes.Buffer
	r := NewJSONReporter(&buf)
	r.Info("ignored\n")
	if buf.Len() != 0 {
		t.Errorf("expected no output from JSONReporter.Info, got %q", buf.String())
	}
}

func TestJSONReporter_Event_DoesNothing(t *testing.T) {
	var buf bytes.Buffer
	r := NewJSONReporter(&buf)
	if err := r.Event("git", nil); err != nil {
		t.Errorf("Event should not error, got: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output from JSONReporter.Event, got %q", buf.String())
	}
}

func TestStreamReporter_Info_DoesNothing(t *testing.T) {
	var buf bytes.Buffer
	r := NewStreamReporter(&buf)
	r.Info("ignored\n")
	if buf.Len() != 0 {
		t.Errorf("expected no output from StreamReporter.Info, got %q", buf.String())
	}
}

func TestConcurrentSafety(t *testing.T) {
	// Smoke test: goroutines writing via Reporter don't race.
	var buf bytes.Buffer
	r := NewStreamReporter(&buf)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r.Event("e", map[string]any{"i": i})
		}(i)
	}
	wg.Wait()

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 20 {
		t.Errorf("expected 20 lines, got %d", len(lines))
	}
}

type testErr struct {
	msg string
}

func (e *testErr) Error() string { return e.msg }
