package bundle

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/RiddhiKatarki/ctx/internal/schema"
	"github.com/RiddhiKatarki/ctx/internal/summary"
	"github.com/RiddhiKatarki/ctx/pkg/types"
)

// Bundle is the in-memory representation of all archive contents.
// Contents holds file contents embedded under the "contents/" prefix
// when the producing side used --include-contents.
type Bundle struct {
	Manifest types.Manifest
	Metadata types.Metadata
	Git      types.GitMetadata
	Summary  types.Summary
	Prompts  []types.Prompt
	Files    []string
	Diff     string
	Contents map[string][]byte
}

// Build assembles a Bundle from a Snapshot and a generated Summary.
// contents is the optional embedded file map (path → bytes); pass nil
// to produce a metadata-only bundle.
func Build(snapshot types.Snapshot, summ *types.Summary, contents map[string][]byte) *Bundle {
	m := types.Manifest{
		Version:          schema.BundleVersion,
		CreatedAt:        time.Now().UTC(),
		Tool:             schema.ToolName,
		IncludesContents: len(contents) > 0,
	}
	return &Bundle{
		Manifest: m,
		Metadata: snapshot.Metadata,
		Git:      snapshot.Git,
		Summary:  *summ,
		Prompts:  snapshot.Prompts,
		Files:    snapshot.Files,
		Diff:     snapshot.Diff,
		Contents: contents,
	}
}

// Serialize converts the Bundle into a map of {filename: bytes}
// suitable for writing into a .ctx archive.
func (b *Bundle) Serialize() (map[string][]byte, error) {
	files := make(map[string][]byte)

	manifestJSON, err := json.MarshalIndent(b.Manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal manifest: %w", err)
	}
	files[schema.ManifestFile] = manifestJSON

	metadataJSON, err := json.MarshalIndent(b.Metadata, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}
	files[schema.MetadataFile] = metadataJSON

	gitJSON, err := json.MarshalIndent(b.Git, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal git metadata: %w", err)
	}
	files[schema.GitFile] = gitJSON

	summaryMD := summary.RenderMarkdown(&b.Summary)
	files[schema.SummaryFile] = []byte(summaryMD)

	promptsJSON, err := json.MarshalIndent(b.Prompts, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal prompts: %w", err)
	}
	files[schema.PromptsFile] = promptsJSON

	if b.Files == nil {
		b.Files = []string{}
	}
	filesJSON, err := json.MarshalIndent(b.Files, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal files list: %w", err)
	}
	files[schema.FilesFile] = filesJSON

	files[schema.PatchFile] = []byte(b.Diff)

	for path, content := range b.Contents {
		// Sanitise path anchored at a virtual root so "../" segments
		// are stripped. We remove the leading "/" so the entry sits
		// cleanly under ContentsPrefix without a double slash.
		clean := filepath.Clean(string(filepath.Separator) + path)
		clean = strings.TrimPrefix(clean, string(filepath.Separator))
		files[schema.ContentsPrefix+clean] = content
	}

	return files, nil
}

// Deserialize parses a map of {filename: bytes} into a Bundle,
// validating the manifest version in the process. Contents entries
// (under ContentsPrefix) are loaded into the bundle.Contents map.
func Deserialize(files map[string][]byte) (*Bundle, error) {
	var manifest types.Manifest
	if data, ok := files[schema.ManifestFile]; ok {
		if err := json.Unmarshal(data, &manifest); err != nil {
			return nil, fmt.Errorf("failed to parse manifest.json: %w", err)
		}
	} else {
		return nil, fmt.Errorf("manifest.json not found in bundle")
	}

	if err := schema.ValidateManifest(&manifest); err != nil {
		return nil, err
	}

	var metadata types.Metadata
	if data, ok := files[schema.MetadataFile]; ok {
		if err := json.Unmarshal(data, &metadata); err != nil {
			return nil, fmt.Errorf("failed to parse metadata.json: %w", err)
		}
	}

	var gitMeta types.GitMetadata
	if data, ok := files[schema.GitFile]; ok {
		if err := json.Unmarshal(data, &gitMeta); err != nil {
			return nil, fmt.Errorf("failed to parse git.json: %w", err)
		}
	}

	var prompts []types.Prompt
	if data, ok := files[schema.PromptsFile]; ok {
		if err := json.Unmarshal(data, &prompts); err != nil {
			return nil, fmt.Errorf("failed to parse prompts.json: %w", err)
		}
	}

	var fileList []string
	if data, ok := files[schema.FilesFile]; ok {
		if err := json.Unmarshal(data, &fileList); err != nil {
			return nil, fmt.Errorf("failed to parse files.json: %w", err)
		}
	}

	diff := ""
	if data, ok := files[schema.PatchFile]; ok {
		diff = string(data)
	}

	var summ types.Summary
	if data, ok := files[schema.SummaryFile]; ok {
		parsed := summary.ParseMarkdownSummary(string(data))
		summ = *parsed
	}

	contents := make(map[string][]byte)
	for name, data := range files {
		if schema.IsContentsPath(name) {
			rel := name[len(schema.ContentsPrefix):]
			contents[rel] = data
		}
	}

	return &Bundle{
		Manifest: manifest,
		Metadata: metadata,
		Git:      gitMeta,
		Summary:  summ,
		Prompts:  prompts,
		Files:    fileList,
		Diff:     diff,
		Contents: contents,
	}, nil
}
