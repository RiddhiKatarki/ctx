package discovery

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/RiddhiKatarki/ctx/internal/archive"
)

// Entry describes a single .ctx bundle as discovered by List.
// Only cheap-to-read fields are populated — full content extraction
// is done by Info / Inspect.
type Entry struct {
	Path            string `json:"path"`
	Name            string `json:"name"`
	Size            int64  `json:"size"`
	ManifestVersion int    `json:"manifest_version"`
	Tool            string `json:"tool"`
	CreatedAt       string `json:"created_at"`
	ProjectName     string `json:"project_name"`
	Branch          string `json:"branch"`
	Dirty           bool   `json:"dirty"`
	FileCount       int    `json:"file_count"`
	HasDiff         bool   `json:"has_diff"`
	Commit          string `json:"head_commit"`
}

// List scans dir non-recursively for *.ctx files and returns
// a populated Entry slice. Sort: most recently created first.
func List(dir string) ([]Entry, error) {
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
	}

	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("directory not found: %s", dir)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var result []Entry
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".ctx") {
			continue
		}

		fullPath := filepath.Join(dir, name)
		entry, ok := inspectEntry(fullPath, e)
		if !ok {
			// Bundle missing or unreadable — emit a minimal entry
			fi, _ := e.Info()
			result = append(result, Entry{
				Path: fullPath,
				Name: name,
				Size: func() int64 {
					if fi != nil {
						return fi.Size()
					}
					return 0
				}(),
			})
			continue
		}
		result = append(result, entry)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].CreatedAt == result[j].CreatedAt {
			return result[i].Name < result[j].Name
		}
		return result[i].CreatedAt > result[j].CreatedAt
	})

	return result, nil
}

// inspectEntry reads only the manifest, metadata, files, and git
// sections of a single .ctx bundle to populate an Entry.
func inspectEntry(path string, de os.DirEntry) (Entry, bool) {
	entry := Entry{
		Path: path,
		Name: de.Name(),
	}
	if fi, err := de.Info(); err == nil {
		entry.Size = fi.Size()
	}

	if !archive.IsZipFile(path) {
		return entry, false
	}

	peek, err := archive.Peek(path)
	if err != nil {
		return entry, false
	}
	if peek.Manifest != nil {
		entry.ManifestVersion = peek.Manifest.Version
		entry.Tool = peek.Manifest.Tool
		entry.CreatedAt = peek.Manifest.CreatedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	if peek.Metadata != nil {
		entry.ProjectName = peek.Metadata.ProjectName
		entry.Branch = peek.Metadata.Branch
	}
	if peek.Git != nil {
		entry.Dirty = peek.Git.Dirty
		entry.Commit = peek.Git.HeadCommit
	}
	if peek.Files != nil {
		entry.FileCount = len(peek.Files)
	}
	if peek.Diff != nil {
		entry.HasDiff = len(peek.Diff) > 0 && string(peek.Diff) != ""
	}
	return entry, true
}

// InfoResult is the structured output of Info. Fields mirror the
// peek view with light enrichment for tooling consumers.
type InfoResult struct {
	Path       string         `json:"path"`
	Size       int64          `json:"size"`
	Manifest   map[string]any `json:"manifest"`
	Metadata   map[string]any `json:"metadata"`
	Git        map[string]any `json:"git"`
	Files      []string       `json:"files"`
	FileCount  int            `json:"file_count"`
	SummaryLen int            `json:"summary_length"`
	HasDiff    bool           `json:"has_diff"`
	DiffSize   int            `json:"diff_size"`
	Valid      bool           `json:"valid"`
}

// Info returns a populated InfoResult for the given bundle path.
// Errors are returned for unreadable / corrupt bundles.
func Info(path string) (*InfoResult, error) {
	if path == "" {
		return nil, fmt.Errorf("no bundle path specified")
	}

	if !archive.IsZipFile(path) {
		return nil, fmt.Errorf("%s is not a valid .ctx archive (not a ZIP file)", path)
	}

	peek, err := archive.Peek(path)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect bundle: %w", err)
	}

	fi, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat bundle: %w", err)
	}

	result := &InfoResult{
		Path:      path,
		Size:      fi.Size(),
		Files:     peek.Files,
		FileCount: len(peek.Files),
		Valid:     true,
	}

	if peek.Manifest != nil {
		result.Manifest = map[string]any{
			"version":    peek.Manifest.Version,
			"created_at": peek.Manifest.CreatedAt,
			"tool":       peek.Manifest.Tool,
		}
	}
	if peek.Metadata != nil {
		result.Metadata = map[string]any{
			"project_name":    peek.Metadata.ProjectName,
			"branch":          peek.Metadata.Branch,
			"created_at":      peek.Metadata.CreatedAt,
			"generator":       peek.Metadata.Generator,
			"repository_root": peek.Metadata.RepositoryRoot,
			"os":              peek.Metadata.OS,
		}
	}
	if peek.Git != nil {
		result.Git = map[string]any{
			"current_branch": peek.Git.CurrentBranch,
			"head_commit":    peek.Git.HeadCommit,
			"dirty":          peek.Git.Dirty,
			"remote_url":     peek.Git.RemoteURL,
			"current_tag":    peek.Git.CurrentTag,
		}
		result.HasDiff = peek.Git.Dirty && peek.Diff != nil && len(peek.Diff) > 0
	}
	if result.Files == nil {
		result.Files = []string{}
	}

	if peek.Summary != nil {
		result.SummaryLen = len(peek.Summary)
	}
	if peek.Diff != nil {
		result.DiffSize = len(peek.Diff)
		if len(peek.Diff) > 0 {
			result.HasDiff = true
		}
	}

	return result, nil
}

// PrintJSON marshals any value to JSON for CLI output.
func PrintJSON(v any) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}
