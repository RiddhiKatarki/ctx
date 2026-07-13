package reporter

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
)

// Reporter abstracts how a long-running operation reports progress
// and its final result. Three implementations are provided:
//
//   HumanReporter  — printf-style progress to stderr
//   JSONReporter   — single JSON object on success only
//   StreamReporter — one NDJSON line per event (consumed by tools)
//
// All implementations are safe to call from a single goroutine.
// The package does not assume concurrency.
type Reporter interface {
	// Event reports a structured progress event. The name is stable
	// (e.g. "git_metadata", "summary_complete"); data is arbitrary
	// key-value pairs surfaced to consumers.
	Event(name string, data map[string]any) error

	// Info emits a human-readable progress message. Implementations
	// other than HumanReporter return nil without emitting.
	Info(format string, args ...any)

	// Done emits the final result object. For HumanReporter this is
	// typically a no-op (final summary already printed). For JSON
	// and Stream reporters, this is the primary success signal.
	Done(result any) error
}

// Mode describes which Reporter implementation to construct.
// String values map 1:1 to CLI flags.
type Mode string

const (
	ModeHuman  Mode = "human"
	ModeJSON   Mode = "json"
	ModeStream Mode = "stream"
)

// New constructs a Reporter for the given mode. Unknown modes fall
// back to HumanReporter. If stderr/stdout are nil, defaults are used.
func New(mode Mode, stdout, stderr io.Writer) Reporter {
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}

	switch mode {
	case ModeJSON:
		return NewJSONReporter(stdout)
	case ModeStream:
		return NewStreamReporter(stdout)
	default:
		return NewHumanReporter(stderr)
	}
}

// ----------------------------------------------------------------------------
// HumanReporter — verbatim printf-style output to stderr.
// ----------------------------------------------------------------------------

// HumanReporter writes human-friendly progress lines to a writer.
type HumanReporter struct {
	mu  sync.Mutex
	out io.Writer
}

// NewHumanReporter constructs a HumanReporter that writes to out.
// A nil out defaults to os.Stderr.
func NewHumanReporter(out io.Writer) *HumanReporter {
	if out == nil {
		out = os.Stderr
	}
	return &HumanReporter{out: out}
}

// Event is silent in human output. Progress is conveyed through
// the labeled Info() messages emitted by the caller.
func (r *HumanReporter) Event(name string, data map[string]any) error {
	return nil
}

// Info writes a formatted message to the underlying writer.
func (r *HumanReporter) Info(format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	fmt.Fprintf(r.out, format, args...)
}

// Done is a no-op for human output — the final summary has
// already been printed during Event / Info.
func (r *HumanReporter) Done(result any) error {
	return nil
}

// ----------------------------------------------------------------------------
// JSONReporter — single machine-readable object on success.
// ----------------------------------------------------------------------------

// JSONReporter writes a single pretty-printed JSON object describing
// the final result. No progress events are emitted.
type JSONReporter struct {
	mu     sync.Mutex
	out    io.Writer
	result any
	written bool
}

// NewJSONReporter constructs a JSONReporter. A nil out defaults to os.Stdout.
func NewJSONReporter(out io.Writer) *JSONReporter {
	if out == nil {
		out = os.Stdout
	}
	return &JSONReporter{out: out}
}

// Event is a no-op for JSON output — events are not emitted.
func (r *JSONReporter) Event(name string, data map[string]any) error {
	return nil
}

// Info is a no-op for JSON output.
func (r *JSONReporter) Info(format string, args ...any) {}

// Done emits the accumulated result as pretty-printed JSON.
func (r *JSONReporter) Done(result any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.written {
		return nil
	}
	r.result = result
	r.written = true
	enc := json.NewEncoder(r.out)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

// ----------------------------------------------------------------------------
// StreamReporter — one NDJSON line per event + final line.
// ----------------------------------------------------------------------------

// StreamReporter emits newline-delimited JSON: one object per Event().
// The final result is emitted as a "complete" event by Done().
// Errors during streaming are emitted as an "error" event.
type StreamReporter struct {
	mu     sync.Mutex
	out    io.Writer
	enc    *json.Encoder
}

// NewStreamReporter constructs a StreamReporter.
func NewStreamReporter(out io.Writer) *StreamReporter {
	enc := json.NewEncoder(out)
	return &StreamReporter{out: out, enc: enc}
}

// Event emits a single NDJSON line: {"event": name, "data": ...}.
func (r *StreamReporter) Event(name string, data map[string]any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.enc.Encode(map[string]any{
		"event": name,
		"data":  data,
	})
}

// Info is a no-op for stream output.
func (r *StreamReporter) Info(format string, args ...any) {}

// Done emits a "complete" NDJSON line carrying the final result.
func (r *StreamReporter) Done(result any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.enc.Encode(map[string]any{
		"event": "complete",
		"data":  result,
	})
}

// Error emits a structured "error" NDJSON event.
func (r *StreamReporter) Error(name string, err error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.enc.Encode(map[string]any{
		"event": name,
		"data": map[string]any{
			"message": err.Error(),
		},
	})
}
