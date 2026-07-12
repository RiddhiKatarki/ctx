package schema

import (
	"fmt"

	"github.com/context-handoff/ctx/pkg/types"
)

const (
	// BundleVersion is the current version of the .ctx bundle format.
	BundleVersion = 1

	// ToolName identifies the tool that generated the bundle.
	ToolName = "ctx"
)

const (
	ManifestFile = "manifest.json"
	MetadataFile = "metadata.json"
	GitFile      = "git.json"
	SummaryFile  = "summary.md"
	PromptsFile  = "prompts.json"
	FilesFile    = "files.json"
	PatchFile    = "patch.diff"
)

// RequiredFiles lists all files that must be present in a valid bundle.
var RequiredFiles = []string{
	ManifestFile,
	MetadataFile,
	GitFile,
	SummaryFile,
	PromptsFile,
	FilesFile,
	PatchFile,
}

// ValidateManifest checks that a manifest is compatible with this tool version.
func ValidateManifest(m *types.Manifest) error {
	if m == nil {
		return fmt.Errorf("manifest is nil")
	}
	if m.Version > BundleVersion {
		return fmt.Errorf("bundle version %d is newer than supported version %d; please upgrade ctx", m.Version, BundleVersion)
	}
	if m.Version < 1 {
		return fmt.Errorf("invalid bundle version %d", m.Version)
	}
	return nil
}

// ValidateMetadata performs basic sanity checks on project metadata.
func ValidateMetadata(m *types.Metadata) error {
	if m == nil {
		return fmt.Errorf("metadata is nil")
	}
	if m.ProjectName == "" {
		return fmt.Errorf("metadata: project_name is required")
	}
	if m.CreatedAt.IsZero() {
		return fmt.Errorf("metadata: created_at is required")
	}
	return nil
}

// HasRequiredFiles checks whether all required files are present in the given set.
func HasRequiredFiles(present map[string]bool) []string {
	var missing []string
	for _, f := range RequiredFiles {
		if !present[f] {
			missing = append(missing, f)
		}
	}
	return missing
}
