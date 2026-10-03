package source

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	enry "github.com/go-enry/go-enry/v2"

	"github.com/ducnd58233/vibe-agent/runtime/internal/slopaudit/domain"
	"github.com/ducnd58233/vibe-agent/runtime/internal/slopaudit/infra/syntax"
	"github.com/ducnd58233/vibe-agent/runtime/internal/sourcefiles"
)

const (
	DefaultLongFileLines    = 800
	DuplicateLineMinLength  = 24
	DuplicateLineMinRepeats = 3
)

const (
	LanguageText = "Text"
	ParserText   = "go-enry language detection plus gotreesitter syntax parse plus text rules"
)

var proseLanguages = map[string]struct{}{
	"Markdown": {},
	"Text":     {},
	"AsciiDoc": {},
	"Org":      {},
}

func isProseLanguage(language string) bool {
	_, ok := proseLanguages[language]
	return ok
}

// dataLanguages hold configuration and data, never a call. Running the debug
// rule over them scored the word "debug" wherever it appeared in a description
// or a regex, which is how this repository's own vibe-checks.yaml came to be
// flagged for a sentence about skipping prose-only debug noise.
var dataLanguages = map[string]struct{}{
	"YAML": {}, "JSON": {}, "TOML": {}, "INI": {}, "JSON5": {}, "JSONLD": {},
}

func isDataLanguage(language string) bool {
	_, ok := dataLanguages[language]
	return ok
}

var (
	emptyBraceFunction = regexp.MustCompile(`(?i)\b(func|function|fn|def|class|interface|method)\b[^\n{}]*\{\s*\}`)
	emptyPythonBody    = regexp.MustCompile(`(?i)^\s*(async\s+)?def\s+\w+\([^)]*\):\s*$`)
	ignoredCall        = regexp.MustCompile(`(^|[^[:alnum:]_])_\s*=\s*[[:alnum:]_\.]+\s*\(`)
	unfinishedMarker   = regexp.MustCompile(`(?i)\b(todo|fixme|hack|placeholder)\b|not implemented|unimplemented`)
	// A brace pair directly after struct or interface is a Go type literal.
	// map[string]struct{} in a signature is not an empty body, and the rule
	// reported one because "func" appeared earlier on the same line.
	typeLiteralBrace = regexp.MustCompile(`(?i)\b(struct|interface)\{\s*\}`)
	// t.Fatal and friends are assertions. Their message describes what went
	// wrong, so a test about placeholder handling read as unfinished code.
	// The receiver list is deliberately short: log.Fatal must keep matching.
	testAssertionCall  = regexp.MustCompile(`(?i)\b(t|b|tb|f|m)\.(fatal|fatalf|error|errorf|skip|skipf)\b`)
	swallowedErrBlock  = regexp.MustCompile(`(?m)if\s+err\s*!=\s*nil\s*\{\s*\}`)
	swallowedErrReturn = regexp.MustCompile(`(?m)if\s+err\s*!=\s*nil\s*\{\s*return\s*\}`)
	aiTellInComment    = regexp.MustCompile(`(?i)\b(ensure|enhance|leverage|utilize|seamless|robust|comprehensive|delve)\b`)
)

var swallowedErrChecks = []struct {
	pattern *regexp.Regexp
	message string
}{
	{swallowedErrBlock, "error branch is empty"},
	{swallowedErrReturn, "error branch returns without handling"},
}

type syntaxParser interface {
	Parse(path string, source []byte, language string) syntax.Result
}

type Scanner struct {
	workers       int
	longFileLines int
	syntaxParser  syntaxParser
}

func NewScanner(workers int) *Scanner {
	return newScanner(workers, syntax.NewParser())
}

func newScanner(workers int, syntaxParser syntaxParser) *Scanner {
	if workers <= 0 {
		workers = 1
	}
	return &Scanner{workers: workers, longFileLines: DefaultLongFileLines, syntaxParser: syntaxParser}
}

func (s *Scanner) Scan(ctx context.Context, target string) (domain.ScanResult, error) {
	files, err := sourceFiles(ctx, target)
	if err != nil {
		return domain.ScanResult{}, err
	}

	jobs := make(chan string)
	out := make(chan fileResult, len(files))
	var wg sync.WaitGroup
	for i := 0; i < s.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}
				out <- s.scanFile(path)
			}
		}()
	}

sendJobs:
	for _, path := range files {
		select {
		case <-ctx.Done():
			break sendJobs
		case jobs <- path:
		}
	}
	close(jobs)
	wg.Wait()
	close(out)

	result := domain.ScanResult{Summary: domain.ScanSummary{Languages: map[string]int{}, Parser: ParserText}}
	for scanned := range out {
		if scanned.skipped {
			continue
		}
		result.Findings = append(result.Findings, scanned.findings...)
		result.Summary.FilesScanned++
		result.Summary.LinesScanned += scanned.lines
		result.Summary.TreeSitterParsed += scanned.treeParsed
		result.Summary.TreeSitterFailures += scanned.treeFailures
		result.Summary.Languages[scanned.language]++
	}
	return result, ctx.Err()
}

type fileResult struct {
	findings     []domain.Finding
	language     string
	lines        int
	treeParsed   int
	treeFailures int
	skipped      bool
}

// sourceFiles is the shared inventory under a directory target, or the
// target itself when it names one file.
func sourceFiles(ctx context.Context, target string) ([]string, error) {
	info, err := os.Stat(target)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []string{filepath.Clean(target)}, nil
	}
	listed, err := sourcefiles.List(ctx, target)
	if err != nil {
		return nil, err
	}
	files := make([]string, 0, len(listed))
	for _, file := range listed {
		files = append(files, file.Abs)
	}
	return files, nil
}

func sourceLanguage(path string, data []byte) string {
	language := enry.GetLanguage(filepath.ToSlash(path), data)
	if language == "" {
		return LanguageText
	}
	return language
}

func (s *Scanner) scanFile(path string) fileResult {
	info, statErr := os.Stat(path)
	if statErr != nil || info.IsDir() {
		return fileResult{skipped: true}
	}
	data, err := fs.ReadFile(os.DirFS(filepath.Dir(path)), filepath.Base(path))
	if err != nil {
		return fileResult{skipped: true}
	}
	if skipFile(path, data) {
		return fileResult{skipped: true}
	}
	language := sourceLanguage(path, data)
	text := string(data)
	lines := strings.Split(text, "\n")
	findings := s.lineFindings(path, language, lines)
	findings = append(findings, fileFindings(path, text, lines)...)
	treeParsed, treeFailures := 0, 0
	if s.syntaxParser != nil {
		parsed := s.syntaxParser.Parse(path, data, language)
		if parsed.Parsed {
			treeParsed = 1
		}
		if parsed.Error != "" {
			treeFailures = 1
			findings = append(findings, syntaxFinding(path, parsed))
		}
	}
	return fileResult{findings: findings, language: language, lines: len(lines), treeParsed: treeParsed, treeFailures: treeFailures}
}

func skipFile(path string, data []byte) bool {
	return sourcefiles.Sensitive(filepath.Base(path)) || !sourcefiles.Readable(filepath.ToSlash(path), data)
}

func (s *Scanner) lineFindings(path, language string, lines []string) []domain.Finding {
	var findings []domain.Finding
	prose := isProseLanguage(language)
	duplicates := map[string]int{}
	for index, line := range lines {
		lineNumber := index + 1
		lower := strings.ToLower(line)
		trimmed := strings.TrimSpace(line)
		if hasUnfinishedComment(lower) {
			findings = append(findings, finding(path, lineNumber, domain.RuleTodoComment, domain.SeverityLow, "unfinished marker in source text"))
		}
		if ignoredCall.MatchString(line) {
			findings = append(findings, finding(path, lineNumber, domain.RuleIgnoredResult, domain.SeverityMedium, "assignment discards a call result"))
		}
		if !prose && hasDebugOutput(language, lower) {
			findings = append(findings, finding(path, lineNumber, domain.RuleDebugPrint, domain.SeverityLow, "debug output call left in source"))
		}
		if hasPlaceholderAbort(lower) {
			findings = append(findings, finding(path, lineNumber, domain.RulePanicPlaceholder, domain.SeverityHigh, "placeholder abort looks like unfinished code"))
		}
		if !prose && isSourceComment(trimmed) && aiTellInComment.MatchString(line) {
			findings = append(findings, finding(path, lineNumber, domain.RuleAITellComment, domain.SeverityInfo, "comment uses common AI filler wording"))
		}
		if !prose && len(trimmed) >= DuplicateLineMinLength {
			duplicates[trimmed]++
			if duplicates[trimmed] == DuplicateLineMinRepeats {
				findings = append(findings, finding(path, lineNumber, domain.RuleDuplicateLine, domain.SeverityLow, "same non-trivial line appears repeatedly"))
			}
		}
	}
	if len(lines) > s.longFileLines {
		findings = append(findings, finding(path, 1, domain.RuleLongFile, domain.SeverityMedium, "file is longer than the configured audit limit"))
	}
	return findings
}

func fileFindings(path, text string, lines []string) []domain.Finding {
	var findings []domain.Finding
	searchText := maskQuotedText(text)
	if index := emptyDeclarationIndex(searchText); index >= 0 {
		findings = append(findings, finding(path, lineOfIndex(searchText, index), domain.RuleEmptyFunction, domain.SeverityHigh, "empty declaration body"))
	}
	if strings.HasSuffix(strings.ToLower(filepath.Base(path)), ".go") {
		findings = append(findings, swallowedErrorFindings(path, searchText)...)
	}
	for index, line := range lines {
		if !emptyPythonBody.MatchString(line) || index+1 >= len(lines) {
			continue
		}
		next := strings.TrimSpace(lines[index+1])
		if next == "pass" || next == "..." {
			findings = append(findings, finding(path, index+2, domain.RuleEmptyFunction, domain.SeverityHigh, "Python function body is only a placeholder"))
		}
	}
	return findings
}

func swallowedErrorFindings(path, text string) []domain.Finding {
	var findings []domain.Finding
	for _, check := range swallowedErrChecks {
		if check.pattern.FindStringIndex(text) == nil {
			continue
		}
		findings = append(findings, finding(path, firstMatchLine(text, check.pattern), domain.RuleSwallowedError, domain.SeverityMedium, check.message))
	}
	return findings
}

func isSourceComment(trimmed string) bool {
	if trimmed == "" {
		return false
	}
	return strings.HasPrefix(trimmed, "//") ||
		strings.HasPrefix(trimmed, "#") ||
		strings.HasPrefix(trimmed, "/*") ||
		strings.HasPrefix(trimmed, "*") ||
		strings.HasPrefix(trimmed, "<!--")
}

func hasUnfinishedComment(lower string) bool {
	for _, marker := range []string{"//", "#", "/*", "<!--"} {
		if index := strings.Index(lower, marker); index >= 0 {
			return unfinishedMarker.MatchString(lower[index:])
		}
	}
	return false
}

func hasUnfinishedMarker(lower string) bool {
	return unfinishedMarker.MatchString(lower)
}

func hasDebugOutput(language, lower string) bool {
	if isDataLanguage(language) {
		return false
	}
	if !strings.Contains(lower, "debug") && !strings.Contains(lower, "todo") && !strings.Contains(lower, "temporary") {
		return false
	}
	switch language {
	case "Go":
		return strings.Contains(lower, "fmt.print") || strings.Contains(lower, "log.print")
	case "JavaScript", "TypeScript", "Tsx":
		return strings.Contains(lower, "console.log") || strings.Contains(lower, "debugger")
	case "Python":
		return strings.Contains(lower, "print(")
	case "Rust":
		return strings.Contains(lower, "println!") || strings.Contains(lower, "dbg!")
	default:
		return strings.Contains(lower, "print") || strings.Contains(lower, "debug")
	}
}

func hasPlaceholderAbort(lower string) bool {
	if !hasUnfinishedMarker(lower) {
		return false
	}
	if testAssertionCall.MatchString(lower) {
		return false
	}
	return strings.Contains(lower, "panic") || strings.Contains(lower, "throw") || strings.Contains(lower, "raise") || strings.Contains(lower, "fatal")
}

// emptyDeclarationIndex returns the offset of the first genuinely empty
// declaration body, or -1.
//
// Type literals are masked rather than skipped. Discarding a match that ends in
// one loses the body behind it: in `func f() interface{} {}` the regex stops at
// the first brace pair, so rejecting that match hid a real empty function.
// Masking leaves the braces the declaration actually owns.
func emptyDeclarationIndex(text string) int {
	masked := maskTypeLiterals(text)
	loc := emptyBraceFunction.FindStringIndex(masked)
	if loc == nil {
		return -1
	}
	return loc[0]
}

// maskTypeLiterals blanks the braces of struct{} and interface{}, keeping the
// text the same length so offsets still point into the original.
func maskTypeLiterals(text string) string {
	return typeLiteralBrace.ReplaceAllStringFunc(text, func(match string) string {
		return strings.Repeat(" ", len(match))
	})
}

func lineOfIndex(text string, index int) int {
	return strings.Count(text[:index], "\n") + 1
}

func maskQuotedText(text string) string {
	var out strings.Builder
	out.Grow(len(text))
	quote := rune(0)
	escaped := false
	for _, char := range text {
		if quote != 0 {
			if char == '\n' {
				out.WriteRune(char)
			} else {
				out.WriteRune(' ')
			}
			if quote != '`' && escaped {
				escaped = false
				continue
			}
			if quote != '`' && char == '\\' {
				escaped = true
				continue
			}
			if char == quote {
				quote = 0
			}
			continue
		}
		switch char {
		case '\'', '"', '`':
			quote = char
			out.WriteRune(' ')
		default:
			out.WriteRune(char)
		}
	}
	return out.String()
}

func firstMatchLine(text string, pattern *regexp.Regexp) int {
	loc := pattern.FindStringIndex(text)
	if loc == nil {
		return 1
	}
	return lineOfIndex(text, loc[0])
}

func syntaxFinding(path string, parsed syntax.Result) domain.Finding {
	line := parsed.Line
	if line <= 0 {
		line = 1
	}
	message := parsed.Error
	if message == "" {
		message = "tree-sitter parse error"
	}
	return finding(path, line, domain.RuleParseError, domain.SeverityMedium, message)
}

func finding(path string, line int, rule string, severity domain.Severity, message string) domain.Finding {
	return domain.Finding{Path: path, Line: line, Rule: rule, Severity: severity, Message: message}
}
