package secretscan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScan_AWSAccessKey(t *testing.T) {
	s := New()
	content := []byte("AWS_ACCESS_KEY=AKIAIOSFODNN7EXAMPLE\nother content")
	findings := s.Scan(content, "config.env")
	if len(findings) == 0 {
		t.Fatal("expected at least one finding")
	}
	if findings[0].Rule != "aws_access_key" {
		t.Errorf("expected aws_access_key rule, got %s", findings[0].Rule)
	}
	if findings[0].Severity != SeverityHigh {
		t.Errorf("expected high severity, got %s", findings[0].Severity)
	}
}

func TestScan_GitHubPAT(t *testing.T) {
	s := New()
	pat := "ghp_" + strings.Repeat("a", 36)
	content := []byte("token: " + pat)
	findings := s.Scan(content, "config")
	if len(findings) == 0 {
		t.Fatal("expected finding for GitHub PAT")
	}
	hasGitHub := false
	for _, f := range findings {
		if f.Rule == "github_pat" {
			hasGitHub = true
			if f.Preview == pat {
				t.Error("Preview should be redacted")
			}
		}
	}
	if !hasGitHub {
		t.Error("expected github_pat rule")
	}
}

func TestScan_PrivateKey(t *testing.T) {
	s := New()
	content := []byte("-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAA...")
	findings := s.Scan(content, "key.pem")
	if len(findings) == 0 {
		t.Fatal("expected finding for private key")
	}
	if findings[0].Rule != "private_key" {
		t.Errorf("expected private_key, got %s", findings[0].Rule)
	}
}

func TestScan_GenericSecretPair_RealValue(t *testing.T) {
	s := New()
	content := []byte("api_key=sk_test_real_key_value_12345")
	findings := s.Scan(content, "config")
	if len(findings) == 0 {
		t.Fatal("expected finding for generic secret pair")
	}
}

func TestScan_GenericSecretPair_PlaceholderExcluded(t *testing.T) {
	s := New()
	content := []byte("api_key=your_api_key_here\npassword=changeme")
	findings := s.Scan(content, "config")
	for _, f := range findings {
		if f.Rule == "generic_secret_assignment" {
			t.Errorf("placeholder should be filtered, got finding %+v", f)
		}
	}
}

func TestScan_HighEntropy(t *testing.T) {
	s := New()
	// Construct a fake high-entropy base64 string.
	random := "AbCd3fGh4jKl5mNp6qRs7tUv8wXy9zAbCd3fGh4jKl5mNp6qRs7tUv8wXy9z"
	content := []byte("token: " + random)
	findings := s.Scan(content, "config")
	hasEntropy := false
	for _, f := range findings {
		if f.Rule == "high_entropy_string" {
			hasEntropy = true
		}
	}
	if !hasEntropy {
		t.Error("expected high_entropy finding for random-looking string")
	}
}

func TestScan_LowEntropy_NoFinding(t *testing.T) {
	s := New()
	content := []byte("this is just plain english text not a secret")
	findings := s.Scan(content, "readme")
	for _, f := range findings {
		if f.Rule == "high_entropy_string" || f.Rule == "generic_secret_assignment" {
			t.Errorf("unexpected finding for plain text: %+v", f)
		}
	}
}

func TestScan_LineColumn(t *testing.T) {
	s := New()
	content := []byte("line1\nline2\nAKIAIOSFODNN7EXAMPLE\n")
	findings := s.Scan(content, "f")
	if len(findings) == 0 {
		t.Fatal("expected finding")
	}
	if findings[0].Line != 3 {
		t.Errorf("expected line 3, got %d", findings[0].Line)
	}
	if findings[0].Column < 1 {
		t.Errorf("expected col >= 1, got %d", findings[0].Column)
	}
}

func TestRedactMatch_ShortValue(t *testing.T) {
	if RedactMatch("abc") != "***" {
		t.Errorf("expected *** for short value, got %s", RedactMatch("abc"))
	}
}

func TestRedactMatch_LongValue(t *testing.T) {
	long := "AKIAIOSFODNN7EXAMPLE"
	r := RedactMatch(long)
	if strings.Contains(r, "IOSFODNN7EXAMPLE") {
		t.Errorf("expected redaction to hide middle part, got %s", r)
	}
	if !strings.HasPrefix(r, "AKIA") {
		t.Errorf("expected to keep prefix for recognition, got %s", r)
	}
}

func TestSummarise(t *testing.T) {
	findings := []Finding{
		{Rule: "aws_access_key"},
		{Rule: "aws_access_key"},
		{Rule: "github_pat"},
	}
	s := Summarise(findings)
	if s["aws_access_key"] != 2 {
		t.Errorf("expected 2 aws_access_key, got %d", s["aws_access_key"])
	}
	if s["github_pat"] != 1 {
		t.Errorf("expected 1 github_pat, got %d", s["github_pat"])
	}
}

func TestIgnoreFile_LoadAndMatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".ctxignore")
	content := `# secrets and noise
*.pem
config/local.json
!important.go
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	ig, err := LoadIgnoreFile(path)
	if err != nil {
		t.Fatalf("LoadIgnoreFile: %v", err)
	}

	tests := []struct {
		path    string
		ignored bool
	}{
		{"cert.pem", true},
		{"config/local.json", true},
		{"important.go", false},  // negated
		{"main.go", false},        // not in patterns
		{"docs/readme.md", false}, // not in patterns
	}

	for _, tt := range tests {
		if got := ig.Match(tt.path); got != tt.ignored {
			t.Errorf("Match(%q) = %v, expected %v", tt.path, got, tt.ignored)
		}
	}
}

func TestIgnoreFile_EmptyIgnore(t *testing.T) {
	ig := EmptyIgnore()
	if ig.Match("anything") {
		t.Error("empty ignore should not match anything")
	}
}

func TestIgnoreFile_NilSafe(t *testing.T) {
	var ig *IgnoreFile
	if ig.Match("x") {
		t.Error("nil ignore should not match")
	}
	if ig.Patterns() != nil && len(ig.Patterns()) > 0 {
		t.Error("nil ignore should have empty patterns")
	}
}

func TestIgnoreFile_CommetnsAndBlankLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".ctxignore")
	if err := os.WriteFile(path, []byte("# comment\n\n*.log\n# another\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	ig, err := LoadIgnoreFile(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if !ig.Match("debug.log") {
		t.Error("expected debug.log to match *.log")
	}
	if ig.Match("main.go") {
		t.Error("main.go should not match")
	}
}

func TestIgnoreFile_NegationAfterExclude(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".ctxignore")
	content := "*.go\n!keep.go\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	ig, _ := LoadIgnoreFile(path)

	if !ig.Match("main.go") {
		t.Error("main.go should match *.go")
	}
	if ig.Match("keep.go") {
		t.Error("keep.go should be negated back in")
	}
}

func TestPatternMatches_BasenameAndFull(t *testing.T) {
	if !patternMatches("*.pem", "certs/server.pem") {
		t.Error("glob *.pem should match basename")
	}
	if !patternMatches("certs/*.pem", "certs/server.pem") {
		t.Error("glob certs/*.pem should match full path")
	}
	if patternMatches("*.pem", "certs/server.key") {
		t.Error("*.pem should not match .key")
	}
}

func TestShannonEntropy(t *testing.T) {
	if e := shannonEntropy("aaaa"); e > 0.1 {
		t.Errorf("expected near-zero entropy for repeated chars, got %f", e)
	}
	if e := shannonEntropy("abc123XYZ"); e < 2.5 {
		t.Errorf("expected moderate entropy, got %f", e)
	}
	if e := shannonEntropy("Rh43Xz2JKLmN9QrstuVWxABCDEFGHiJK"); e < 4.5 {
		t.Errorf("expected high entropy, got %f", e)
	}
}

func TestScan_VeniceAPIKey(t *testing.T) {
	// Sanity check that the test data we use for the demo
	// isn't flagged as a "real" rule we care about.
	s := New()
	content := []byte("VENICE_INFERENCE_KEY_kPzicYEUzLZKlIzr5S1CCWbAri7kil0M3rx3O8U-Wo")
	findings := s.Scan(content, "env")
	for _, f := range findings {
		// Venice keys aren't in the default ruleset.
		if f.Rule == "openai_api_key" || f.Rule == "anthropic_api_key" {
			t.Errorf("venice key matched %s, was it supposed to? %+v", f.Rule, f)
		}
	}
}
