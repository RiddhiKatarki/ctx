// Package secretscan detects secrets in textual content using a
// built-in regex rule set plus a high-entropy heuristic.
//
// The package also supports .ctxignore-style file exclusion used
// before scanning.
//
// All public types and methods are safe to call from a single goroutine.
package secretscan

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Severity classifies a Finding by sensitivity.
type Severity string

const (
	SeverityHigh   Severity = "high"
	SeverityMedium Severity = "medium"
	SeverityLow    Severity = "low"
)

// Finding is one detected secret occurrence.
type Finding struct {
	Rule     string   `json:"rule"`
	Severity Severity `json:"severity"`
	Match    string   `json:"match"`            // original (un-redacted) text — redact before exposing
	Preview  string   `json:"preview"`          // safe-to-display redacted preview
	Line     int      `json:"line"`             // 1-indexed line within content
	Column   int      `json:"column"`           // 1-indexed column within line
	Source   string   `json:"source,omitempty"` // human-friendly origin (e.g. filename)
}

// Rule is one named detection pattern.
type Rule struct {
	Name     string
	Severity Severity
	Pattern  *regexp.Regexp
	// Validate optionally refines a regex match (e.g. discards placeholders).
	Validate func(match string) bool
}

// Scanner bundles rules with the entropy heuristic. The zero value
// is not usable — construct via New().
type Scanner struct {
	Rules []Rule
	// EntropyMin is the minimum Shannon entropy for a string to count
	// as a high-entropy candidate. Defaults to 4.5 when zero.
	EntropyMin float64
	// EntropyMinLen is the minimum length a string must be before the
	// entropy heuristic inspects it. Defaults to 40 when zero.
	EntropyMinLen int
}

// New constructs a Scanner with the default rule set.
func New() *Scanner {
	return &Scanner{
		Rules:         defaultRules(),
		EntropyMin:    4.5,
		EntropyMinLen: 40,
	}
}

// DefaultScanner returns the package's default Scanner instance.
func DefaultScanner() *Scanner { return New() }

func defaultRules() []Rule {
	return []Rule{
		awsAccessKey(),
		awsSecretKey(),
		githubPAT(),
		githubFineGrainedPAT(),
		openAIAPIKey(),
		anthropicAPIKey(),
		googleAPIKey(),
		slackToken(),
		genericSecretPair(),
		privateKey(),
	}
}

// HighEntropy returns a Rule for the entropy-based heuristic.
func (s *Scanner) HighEntropy() Rule {
	return Rule{
		Name: "high_entropy_string", Severity: SeverityLow,
		Pattern: regexp.MustCompile(`[A-Za-z0-9+/=_\-]{40,}`),
		Validate: func(match string) bool {
			if len(match) < s.EntropyMinLen {
				return false
			}
			if shannonEntropy(match) < s.EntropyMin {
				return false
			}
			var hasUpper, hasLower, hasDigit bool
			for _, r := range match {
				switch {
				case r >= 'A' && r <= 'Z':
					hasUpper = true
				case r >= 'a' && r <= 'z':
					hasLower = true
				case r >= '0' && r <= '9':
					hasDigit = true
				}
			}
			return hasUpper && hasLower && hasDigit
		},
	}
}

func awsAccessKey() Rule {
	return Rule{Name: "aws_access_key", Severity: SeverityHigh,
		Pattern: regexp.MustCompile(`AKIA[0-9A-Z]{16}`)}
}

func awsSecretKey() Rule {
	return Rule{Name: "aws_secret_key", Severity: SeverityHigh,
		Pattern: regexp.MustCompile(`(?i)aws.{0,20}?[''"\s:=]+[A-Za-z0-9/+=]{40}`)}
}

func githubPAT() Rule {
	return Rule{Name: "github_pat", Severity: SeverityHigh,
		Pattern: regexp.MustCompile(`ghp_[A-Za-z0-9]{36}`)}
}

func githubFineGrainedPAT() Rule {
	return Rule{Name: "github_fine_grained_pat", Severity: SeverityHigh,
		Pattern: regexp.MustCompile(`github_pat_[A-Za-z0-9_]{82}`)}
}

func openAIAPIKey() Rule {
	return Rule{Name: "openai_api_key", Severity: SeverityHigh,
		Pattern: regexp.MustCompile(`sk-[A-Za-z0-9]{20,}T3BlbkFJ[A-Za-z0-9]{20,}|sk-(?:proj-)?[A-Za-z0-9_-]{20,}`)}
}

func anthropicAPIKey() Rule {
	return Rule{Name: "anthropic_api_key", Severity: SeverityHigh,
		Pattern: regexp.MustCompile(`sk-ant-[A-Za-z0-9-]{32,}`)}
}

func googleAPIKey() Rule {
	return Rule{Name: "google_api_key", Severity: SeverityHigh,
		Pattern: regexp.MustCompile(`AIza[0-9A-Za-z\-_]{35}`)}
}

func slackToken() Rule {
	return Rule{Name: "slack_token", Severity: SeverityHigh,
		Pattern: regexp.MustCompile(`xox[baprs]-[A-Za-z0-9-]{10,}`)}
}

func genericSecretPair() Rule {
	return Rule{
		Name: "generic_secret_assignment", Severity: SeverityMedium,
		Pattern: regexp.MustCompile(`(?i)(api[_-]?key|secret|password|token|access[_-]?key|private[_-]?key)\s*[=:]\s*['"]?([A-Za-z0-9_\-/.+=]{16,})['"]?`),
		Validate: func(match string) bool {
			lower := strings.ToLower(match)
			for _, bad := range []string{"your_", "example", "placeholder", "changeme", "xxx", "todo"} {
				if strings.Contains(lower, bad) {
					return false
				}
			}
			return true
		},
	}
}

func privateKey() Rule {
	return Rule{
		Name: "private_key", Severity: SeverityHigh,
		Pattern: regexp.MustCompile(`-----BEGIN (RSA |EC |DSA |OPENSSH |PGP )?PRIVATE KEY-----`),
	}
}

// Scan inspects content with all configured rules plus the entropy
// heuristic, returning Findings with line/column locations.
func (s *Scanner) Scan(content []byte, source string) []Finding {
	if s == nil {
		s = DefaultScanner()
	}
	text := string(content)
	var findings []Finding
	rules := append([]Rule{}, s.Rules...)
	rules = append(rules, s.HighEntropy())

	for _, rule := range rules {
		for _, idx := range rule.Pattern.FindAllStringIndex(text, -1) {
			match := text[idx[0]:idx[1]]
			if rule.Validate != nil && !rule.Validate(match) {
				continue
			}
			line, col := lineColumn(text, idx[0])
			findings = append(findings, Finding{
				Rule:     rule.Name,
				Severity: rule.Severity,
				Match:    match,
				Preview:  RedactMatch(match),
				Line:     line,
				Column:   col,
				Source:   source,
			})
		}
	}
	return findings
}

// RedactMatch returns a partially-redacted preview that preserves
// enough of the secret for the author to recognise it without
// leaking the full value.
func RedactMatch(s string) string {
	if len(s) <= 8 {
		return "***"
	}
	visible := 4
	if visible > len(s)/2 {
		visible = len(s) / 2
	}
	return s[:visible] + strings.Repeat("*", 8) + s[len(s)-2:]
}

// Summary returns an aggregate count grouped by rule name.
type Summary map[string]int

// Summarise counts findings by rule.
func Summarise(findings []Finding) Summary {
	out := make(Summary)
	for _, f := range findings {
		out[f.Rule]++
	}
	return out
}

// IgnoreFile holds parsed .ctxignore entries supporting glob patterns
// with negation. Patterns follow gitignore-like semantics (without
// full directory anchoring; caller normalises paths).
type IgnoreFile struct {
	patterns []string
	negative []string
	raw      []string
}

// LoadIgnoreFile parses a .ctxignore-style file. Lines starting with
// '#' (after optional whitespace) are comments. Blank lines are
// skipped. Patterns starting with '!' are negation patterns.
func LoadIgnoreFile(path string) (*IgnoreFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	ig := &IgnoreFile{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		ig.raw = append(ig.raw, line)
		if strings.HasPrefix(line, "!") {
			ig.negative = append(ig.negative, strings.TrimPrefix(line, "!"))
			continue
		}
		ig.patterns = append(ig.patterns, line)
	}
	if err := scanner.Err(); err != nil {
		return ig, fmt.Errorf("read %s: %w", path, err)
	}
	return ig, nil
}

// EmptyIgnore is a no-op IgnoreFile used when no .ctxignore exists.
func EmptyIgnore() *IgnoreFile { return &IgnoreFile{} }

// Match returns true when path should be ignored. Supports negation
// via '!pattern'. The matcher's pattern semantics: a pattern matches
// the path basename; a pattern containing '/' is matched against the
// full relative path.
func (ig *IgnoreFile) Match(path string) bool {
	if ig == nil || path == "" {
		return false
	}
	matched := false
	// Check positive patterns.
	for _, p := range ig.patterns {
		if patternMatches(p, path) {
			matched = true
			break
		}
	}
	if !matched {
		return false
	}
	// Apply negations.
	for _, p := range ig.negative {
		if patternMatches(p, path) {
			return false
		}
	}
	return true
}

// Patterns returns a copy of the parsed patterns.
func (ig *IgnoreFile) Patterns() []string {
	if ig == nil {
		return nil
	}
	out := make([]string, len(ig.raw))
	copy(out, ig.raw)
	return out
}

func patternMatches(pattern, path string) bool {
	if !strings.Contains(pattern, "/") {
		// Basename match.
		base := filepath.Base(path)
		if ok, _ := filepath.Match(pattern, base); ok {
			return true
		}
		return false
	}
	// Full path match — try direct, also try with leading "./".
	for _, candidate := range []string{path, "./" + path} {
		if ok, _ := filepath.Match(pattern, candidate); ok {
			return true
		}
	}
	return false
}

// lineColumn computes 1-indexed line/column for a byte offset in text.
func lineColumn(text string, offset int) (int, int) {
	line, col := 1, 1
	for i := 0; i < offset && i < len(text); i++ {
		if text[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return line, col
}

// shannonEntropy returns the per-character Shannon entropy in bits.
func shannonEntropy(s string) float64 {
	if len(s) == 0 {
		return 0
	}
	counts := make(map[rune]int)
	for _, r := range s {
		counts[r]++
	}
	var h float64
	ln := float64(len(s))
	for _, c := range counts {
		p := float64(c) / ln
		h -= p * math.Log2(p)
	}
	return h
}
