package types

import "time"

// Manifest describes the bundle version and provenance.
type Manifest struct {
	Version   int       `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	Tool      string    `json:"tool"`
}

// Metadata holds general project metadata captured at export time.
type Metadata struct {
	ProjectName    string    `json:"project_name"`
	Branch         string    `json:"branch"`
	CreatedAt      time.Time `json:"created_at"`
	Generator      string    `json:"generator"`
	RepositoryRoot string    `json:"repository_root"`
	OS             string    `json:"os"`
}

// GitMetadata captures the Git state at export time.
type GitMetadata struct {
	CurrentBranch string `json:"current_branch"`
	HeadCommit    string `json:"head_commit"`
	Dirty         bool   `json:"dirty"`
	RemoteURL     string `json:"remote_url,omitempty"`
	CurrentTag    string `json:"current_tag,omitempty"`
}

// Prompt represents a single prompt entry in the conversation history.
type Prompt struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Summary is the structured AI-generated project summary.
type Summary struct {
	CurrentObjective         string `json:"current_objective"`
	CompletedWork            string `json:"completed_work"`
	RemainingTasks           string `json:"remaining_tasks"`
	KnownBugs                string `json:"known_bugs"`
	ArchitectureDecisions    string `json:"architecture_decisions"`
	FilesToReadFirst         string `json:"files_to_read_first"`
	PreviousFailedApproaches string `json:"previous_failed_approaches"`
	SuggestedNextPrompt      string `json:"suggested_next_prompt"`
	EstimatedReadingTime     string `json:"estimated_reading_time"`
}

// Snapshot is the in-memory object the export flow builds first.
// Everything downstream operates on this object.
type Snapshot struct {
	Metadata Metadata
	Git      GitMetadata
	Prompts  []Prompt
	Files    []string
	Diff     string
}
