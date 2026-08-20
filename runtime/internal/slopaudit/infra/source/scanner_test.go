package source

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/slopaudit/domain"
)

func TestScannerFindsSignalsAcrossLanguages(t *testing.T) {
	dir := filepath.Join("testdata", "languages")
	want := fixtureCount(t, dir)

	result, err := NewScanner(2).Scan(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}

	if result.Summary.FilesScanned != want {
		t.Fatalf("files scanned = %d, want %d", result.Summary.FilesScanned, want)
	}
	if result.Summary.TreeSitterParsed < 3 {
		t.Fatalf("tree-sitter parsed = %d, want at least Go, TSX, and YAML", result.Summary.TreeSitterParsed)
	}
	for _, language := range []string{"Go", "TypeScript", "Python", "SCSS", "PHP", "Rust", "Zig", "Vue", "Dockerfile", "YAML"} {
		if result.Summary.Languages[language] < 1 {
			t.Fatalf("language %s missing in summary: %+v", language, result.Summary.Languages)
		}
	}

	seen := map[string]bool{}
	for _, finding := range result.Findings {
		seen[finding.Rule] = true
	}
	for _, rule := range []string{domain.RuleEmptyFunction, domain.RuleIgnoredResult, domain.RuleDebugPrint, domain.RulePanicPlaceholder, domain.RuleSwallowedError} {
		if !seen[rule] {
			t.Fatalf("missing rule %s in findings: %+v", rule, result.Findings)
		}
	}
}

func TestScannerSkipsVendorDirectoryByEnryRules(t *testing.T) {
	dir := t.TempDir()
	vendorDir := filepath.Join(dir, "node_modules", "pkg")
	if err := os.MkdirAll(vendorDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vendorDir, "ignored.js"), []byte("console.log('debug temporary')"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "kept.js"), []byte("console.log('debug temporary')"), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := NewScanner(1).Scan(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if result.Summary.FilesScanned != 1 {
		t.Fatalf("files scanned = %d, want 1: %+v", result.Summary.FilesScanned, result.Summary)
	}
}

func TestScannerIncludesUnknownTextAndSkipsBinary(t *testing.T) {
	dir := t.TempDir()
	textPath := filepath.Join(dir, "custom.frontend")
	if err := os.WriteFile(textPath, []byte("component debug temporary\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	binaryPath := filepath.Join(dir, "image.png")
	if err := os.WriteFile(binaryPath, []byte{0, 1, 2, 3}, 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := NewScanner(1).Scan(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if result.Summary.Languages[LanguageText] != 1 {
		t.Fatalf("summary = %+v", result.Summary)
	}
	if result.Summary.FilesScanned != 1 {
		t.Fatalf("files scanned = %d, want 1", result.Summary.FilesScanned)
	}
}

func TestScannerScoresByDensity(t *testing.T) {
	findings := []domain.Finding{{Severity: domain.SeverityHigh}, {Severity: domain.SeverityHigh}}
	shortScore := domain.Score(findings, domain.MinimumScoredLines)
	longScore := domain.Score(findings, domain.MinimumScoredLines*10)
	if shortScore <= longScore {
		t.Fatalf("shortScore = %d, longScore = %d", shortScore, longScore)
	}
}

func TestScannerReportsTreeSitterParseErrors(t *testing.T) {
	dir := t.TempDir()
	badPath := filepath.Join(dir, "broken.tsx")
	if err := os.WriteFile(badPath, []byte("export function Broken() { return <div>\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := NewScanner(1).Scan(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if result.Summary.TreeSitterParsed != 1 {
		t.Fatalf("tree-sitter parsed = %d, want 1", result.Summary.TreeSitterParsed)
	}
	if result.Summary.TreeSitterFailures != 1 {
		t.Fatalf("tree-sitter failures = %d, want 1", result.Summary.TreeSitterFailures)
	}
	for _, finding := range result.Findings {
		if finding.Rule == domain.RuleParseError {
			return
		}
	}
	t.Fatalf("missing parse error finding: %+v", result.Findings)
}

func TestScannerFindsAITellCommentAndSwallowedError(t *testing.T) {
	dir := filepath.Join("testdata", "ai-tell")

	result, err := NewScanner(1).Scan(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, finding := range result.Findings {
		seen[finding.Rule] = true
	}
	if !seen[domain.RuleAITellComment] {
		t.Fatalf("missing ai_tell_comment: %+v", result.Findings)
	}
	if !seen[domain.RuleSwallowedError] {
		t.Fatalf("missing swallowed_error: %+v", result.Findings)
	}
}

// fixtureCount reports how many fixture files a directory holds.
//
// The fixtures live under testdata/ because a walk skips that directory by
// enry's vendor rules, so the repository's own audit does not score them. They
// used to be raw string literals in this file, and the line rules read string
// bodies, so the audit counted four high-severity placeholder aborts and a
// debug print against fixtures that were doing exactly their job.
//
// A scan still reaches them because the skip applies to directories the walk
// descends into, not to the directory a scan is pointed at.
func fixtureCount(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	files := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			files++
		}
	}
	if files == 0 {
		t.Fatalf("no fixtures in %s", dir)
	}
	return files
}

// fixtureLines reads sample lines from testdata.
//
// They are files rather than string literals for the same reason the language
// fixtures are: the repository audits itself, so a sample written to prove a
// rule fires is caught by that rule when it sits in this file. testdata is
// excluded from a walk, so the samples stay readable without being scored.
//
// This comment learned it the hard way: naming the sample verbatim here made
// the explanation a finding of its own.
func fixtureLines(t *testing.T, name string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Clean(filepath.Join("testdata", "lines", name+".txt")))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, line := range strings.Split(string(raw), "\n") {
		if trimmed := strings.TrimRight(line, "\r"); strings.TrimSpace(trimmed) != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// A data language has no debug print call to leave behind. Running the rule
// there scored the word "debug" in a YAML description, which is how the
// repository's own vibe-checks.yaml came to be flagged for a sentence about
// skipping prose-only debug noise.
func TestADataLanguageHasNoDebugOutput(t *testing.T) {
	for _, language := range []string{"YAML", "JSON", "TOML", "INI"} {
		for _, line := range fixtureLines(t, "data-language-prose") {
			if hasDebugOutput(language, strings.ToLower(line)) {
				t.Errorf("%s: prose or a pattern was read as a debug call: %s", language, line)
			}
		}
	}

	// The languages that do have one keep it.
	for _, line := range fixtureLines(t, "debug-calls") {
		if !hasDebugOutput("Go", strings.ToLower(line)) {
			t.Errorf("a real Go debug print stopped being detected: %s", line)
		}
	}
}

// struct{} and interface{} are type literals, not empty declaration bodies. The
// regex read the {} in a map[string]struct{} signature as an empty function
// because "func" appeared earlier on the line.
func TestAnEmptyTypeLiteralIsNotAnEmptyDeclaration(t *testing.T) {
	for _, line := range fixtureLines(t, "type-literals") {
		if emptyDeclarationIndex(line) >= 0 {
			t.Errorf("read a type literal as an empty declaration: %s", line)
		}
	}
	for _, line := range fixtureLines(t, "empty-declarations") {
		if emptyDeclarationIndex(line) < 0 {
			t.Errorf("stopped detecting a genuinely empty declaration: %s", line)
		}
	}
}

// t.Fatal is an assertion, and its message describes what went wrong. Counting
// it as an abort verb turned a test about placeholder handling into a
// high-severity finding about unfinished code.
func TestATestAssertionIsNotAPlaceholderAbort(t *testing.T) {
	for _, line := range fixtureLines(t, "test-assertions") {
		if hasPlaceholderAbort(strings.ToLower(line)) {
			t.Errorf("read a test assertion as a placeholder abort: %s", line)
		}
	}
	for _, line := range fixtureLines(t, "placeholder-aborts") {
		if !hasPlaceholderAbort(strings.ToLower(line)) {
			t.Errorf("stopped detecting a real placeholder abort: %s", line)
		}
	}
}
