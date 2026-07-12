package schema

import (
	"testing"
	"time"

	"github.com/RiddhiKatarki/ctx/pkg/types"
)

func TestValidateManifest_ValidVersion(t *testing.T) {
	m := &types.Manifest{Version: 1, CreatedAt: time.Now(), Tool: "ctx"}
	if err := ValidateManifest(m); err != nil {
		t.Errorf("expected no error for version 1, got: %v", err)
	}
}

func TestValidateManifest_NilManifest(t *testing.T) {
	if err := ValidateManifest(nil); err == nil {
		t.Error("expected error for nil manifest")
	}
}

func TestValidateManifest_FutureVersion(t *testing.T) {
	m := &types.Manifest{Version: 999, CreatedAt: time.Now(), Tool: "ctx"}
	if err := ValidateManifest(m); err == nil {
		t.Error("expected error for future version")
	}
}

func TestValidateManifest_InvalidVersion(t *testing.T) {
	m := &types.Manifest{Version: 0, CreatedAt: time.Now(), Tool: "ctx"}
	if err := ValidateManifest(m); err == nil {
		t.Error("expected error for version 0")
	}
}

func TestValidateMetadata_Valid(t *testing.T) {
	m := &types.Metadata{
		ProjectName: "test-project",
		CreatedAt:   time.Now(),
	}
	if err := ValidateMetadata(m); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidateMetadata_MissingProjectName(t *testing.T) {
	m := &types.Metadata{CreatedAt: time.Now()}
	if err := ValidateMetadata(m); err == nil {
		t.Error("expected error for missing project name")
	}
}

func TestValidateMetadata_NilMetadata(t *testing.T) {
	if err := ValidateMetadata(nil); err == nil {
		t.Error("expected error for nil metadata")
	}
}

func TestHasRequiredFiles_AllPresent(t *testing.T) {
	present := make(map[string]bool)
	for _, f := range RequiredFiles {
		present[f] = true
	}
	missing := HasRequiredFiles(present)
	if len(missing) != 0 {
		t.Errorf("expected no missing files, got: %v", missing)
	}
}

func TestHasRequiredFiles_SomeMissing(t *testing.T) {
	present := map[string]bool{
		ManifestFile: true,
		MetadataFile: true,
	}
	missing := HasRequiredFiles(present)
	if len(missing) == 0 {
		t.Error("expected missing files")
	}
}

func TestConstants(t *testing.T) {
	if BundleVersion != 1 {
		t.Errorf("expected BundleVersion=1, got %d", BundleVersion)
	}
	if ToolName != "ctx" {
		t.Errorf("expected ToolName=ctx, got %s", ToolName)
	}
	if ManifestFile != "manifest.json" {
		t.Errorf("expected manifest.json, got %s", ManifestFile)
	}
	if PatchFile != "patch.diff" {
		t.Errorf("expected patch.diff, got %s", PatchFile)
	}
}
