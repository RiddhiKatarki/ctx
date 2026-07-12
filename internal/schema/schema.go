package schema

import (
	"fmt"

	"github.com/RiddhiKatarki/ctx/pkg/types"
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

	// ContentsPrefix is the directory under which embedded file
	// contents live when --include-contents is used.
	ContentsPrefix = "contents/"
)

// IsContentsPath reports whether archive entry name is a contents
// entry (under ContentsPrefix). Used to read non-required files
// from a V2 bundle without disrupting V1 readers.
func IsContentsPath(name string) bool {
	return len(name) > len(ContentsPrefix) && name[:len(ContentsPrefix)] == ContentsPrefix
}

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
