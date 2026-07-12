package archive

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/RiddhiKatarki/ctx/internal/schema"
	"github.com/RiddhiKatarki/ctx/pkg/types"
)

// EnsureCtxExtension ensures the given path ends with .ctx,
// appending it if necessary.
func EnsureCtxExtension(path string) string {
	if strings.HasSuffix(path, ".ctx") {
		return path
	}
	return path + ".ctx"
}

// Create writes a ZIP archive with .ctx extension containing the given files.
// The files map is {filename: content}.
func Create(path string, files map[string][]byte) error {
	path = EnsureCtxExtension(path)

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	out, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create archive file: %w", err)
	}
	defer out.Close()

	zw := zip.NewWriter(out)

	for _, filename := range schema.RequiredFiles {
		content, ok := files[filename]
		if !ok {
			continue
		}
		w, err := zw.Create(filename)
		if err != nil {
			return fmt.Errorf("failed to create zip entry %s: %w", filename, err)
		}
		if _, err := w.Write(content); err != nil {
			return fmt.Errorf("failed to write zip entry %s: %w", filename, err)
		}
	}

	if err := zw.Close(); err != nil {
		return fmt.Errorf("failed to close archive: %w", err)
	}

	return nil
}

// Extract reads a .ctx ZIP archive and returns a map of {filename: content}.
func Extract(path string) (map[string][]byte, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open archive %s: %w", path, err)
	}
	defer r.Close()

	files := make(map[string][]byte)
	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open zip entry %s: %w", f.Name, err)
		}
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read zip entry %s: %w", f.Name, err)
		}
		files[f.Name] = content
	}

	present := make(map[string]bool)
	for name := range files {
		present[name] = true
	}
	missing := schema.HasRequiredFiles(present)
	if len(missing) > 0 {
		return nil, fmt.Errorf("archive is missing required files: %s", strings.Join(missing, ", "))
	}

	return files, nil
}

// PeekResult holds the subset of files read by Peek for fast inspection.
type PeekResult struct {
	Manifest *types.Manifest
	Metadata *types.Metadata
	Git      *types.GitMetadata
	Summary  []byte
	Diff     []byte
	Files    []string
}

// Peek opens a .ctx archive and reads only the files needed for inspection:
// manifest.json, metadata.json, git.json, summary.md, files.json, and patch.diff.
// This avoids full extraction for fast display.
func Peek(path string) (*PeekResult, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open archive %s: %w", path, err)
	}
	defer r.Close()

	result := &PeekResult{}

	for _, f := range r.File {
		switch f.Name {
		case schema.ManifestFile:
			content, err := readZipEntry(f)
			if err != nil {
				return nil, err
			}
			var manifest types.Manifest
			if err := json.Unmarshal(content, &manifest); err != nil {
				return nil, fmt.Errorf("failed to parse manifest: %w", err)
			}
			result.Manifest = &manifest

		case schema.MetadataFile:
			content, err := readZipEntry(f)
			if err != nil {
				return nil, err
			}
			var metadata types.Metadata
			if err := json.Unmarshal(content, &metadata); err != nil {
				return nil, fmt.Errorf("failed to parse metadata: %w", err)
			}
			result.Metadata = &metadata

		case schema.GitFile:
			content, err := readZipEntry(f)
			if err != nil {
				return nil, err
			}
			var git types.GitMetadata
			if err := json.Unmarshal(content, &git); err != nil {
				return nil, fmt.Errorf("failed to parse git: %w", err)
			}
			result.Git = &git

		case schema.SummaryFile:
			content, err := readZipEntry(f)
			if err != nil {
				return nil, err
			}
			result.Summary = content

		case schema.PatchFile:
			content, err := readZipEntry(f)
			if err != nil {
				return nil, err
			}
			result.Diff = content

		case schema.FilesFile:
			content, err := readZipEntry(f)
			if err != nil {
				return nil, err
			}
			var files []string
			if err := json.Unmarshal(content, &files); err != nil {
				return nil, fmt.Errorf("failed to parse files list: %w", err)
			}
			result.Files = files
		}
	}

	if result.Manifest == nil {
		return nil, fmt.Errorf("manifest.json not found in archive")
	}
	if err := schema.ValidateManifest(result.Manifest); err != nil {
		return nil, err
	}

	return result, nil
}

// ExtractToDir extracts a .ctx archive to the given output directory.
func ExtractToDir(path, outDir string) error {
	r, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("failed to open archive %s: %w", path, err)
	}
	defer r.Close()

	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	for _, f := range r.File {
		outPath := filepath.Join(outDir, f.Name)
		if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", f.Name, err)
		}
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("failed to open zip entry %s: %w", f.Name, err)
		}
		out, err := os.Create(outPath)
		if err != nil {
			rc.Close()
			return fmt.Errorf("failed to create file %s: %w", outPath, err)
		}
		if _, err := io.Copy(out, rc); err != nil {
			rc.Close()
			out.Close()
			return fmt.Errorf("failed to write file %s: %w", outPath, err)
		}
		rc.Close()
		out.Close()
	}

	return nil
}

// Size returns the file size of a .ctx archive in bytes.
func Size(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func readZipEntry(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open zip entry %s: %w", f.Name, err)
	}
	defer rc.Close()
	content, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("failed to read zip entry %s: %w", f.Name, err)
	}
	return content, nil
}

// IsZipFile checks whether the given path is a valid ZIP archive.
func IsZipFile(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return bytes.HasPrefix(data, []byte{0x50, 0x4B, 0x03, 0x04})
}
