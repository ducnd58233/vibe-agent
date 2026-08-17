package source

import (
	"bufio"
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
)

const (
	DefaultWorkers          = 4
	MaxFileBytes            = 1024 * 1024
	DefaultLongFileLines    = 800
	DuplicateLineMinLength  = 24
	DuplicateLineMinRepeats = 3
)

const (
	LanguageUnknown = "Unknown"
	LanguageText    = "Text"
	ParserText      = "go-enry language detection plus gotreesitter syntax parse plus text rules"
)

var skippedDirectories = map[string]struct{}{
	".git": {},
}

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

var (
	emptyBraceFunction = regexp.MustCompile(`(?i)\b(func|function|fn|def|class|interface|method)\b[^\n{}]*\{\s*\}`)
	emptyPythonBody    = regexp.MustCompile(`(?i)^\s*(async\s+)?def\s+\w+\([^)]*\):\s*$`)
	ignoredCall        = regexp.MustCompile(`(^|[^[:alnum:]_])_\s*=\s*[[:alnum:]_\.]+\s*\(`)
	unfinishedMarker   = regexp.MustCompile(`(?i)\b(todo|fixme|hack|placeholder)\b|not implemented|unimplemented`)
)

type SyntaxParser interface {
	Parse(path string, source []byte, language string) syntax.Result
}

type Scanner struct {
	workers       int
	longFileLines int
	syntaxParser  SyntaxParser
}

func NewScanner(workers int) *Scanner {
	return NewScannerWithSyntax(workers, syntax.NewParser())
}

func NewScannerWithSyntax(workers int, syntaxParser SyntaxParser) *Scanner {
	if workers <= 0 {
		workers = DefaultWorkers
	}
	return &Scanner{workers: workers, longFileLines: DefaultLongFileLines, syntaxParser: syntaxParser}
}

func (s *Scanner) Scan(ctx context.Context, target string) (domain.ScanResult, error) {
	files, err := sourceFiles(target)
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

func sourceFiles(target string) ([]string, error) {
	var files []string
	info, err := os.Stat(target)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []string{filepath.Clean(target)}, nil
	}
	root := filepath.Clean(target)
	if absRoot, err := filepath.Abs(root); err == nil {
		root = absRoot
	}
	ignore := loadGitignore(root)
	err = filepath.WalkDir(target, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			if filepath.Clean(path) == filepath.Clean(target) {
				return err
			}
			if entry != nil && entry.IsDir() {
				// Junctions and symlinked dirs on Windows can make ReadDir fail.
				return filepath.SkipDir
			}
			return nil
		}
		clean := filepath.Clean(path)
		if abs, absErr := filepath.Abs(clean); absErr == nil {
			clean = abs
		}
		if entry.IsDir() {
			if clean != root {
				if _, ok := skippedDirectories[entry.Name()]; ok {
					return filepath.SkipDir
				}
				if enry.IsVendor(filepath.ToSlash(path)) {
					return filepath.SkipDir
				}
				if ignore.skipDir(clean) {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if ignore.skipFile(clean) {
			return nil
		}
		files = append(files, clean)
		return nil
	})
	return files, err
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
	slashPath := filepath.ToSlash(path)
	if sensitiveFile(filepath.Base(path)) {
		return true
	}
	return len(data) > MaxFileBytes ||
		enry.IsBinary(data) ||
		enry.IsImage(slashPath) ||
		enry.IsGenerated(slashPath, data)
}

func sensitiveFile(name string) bool {
	lower := strings.ToLower(name)
	return lower == ".env" ||
		strings.HasPrefix(lower, ".env.") ||
		strings.HasSuffix(lower, ".local") ||
		strings.HasSuffix(lower, ".local.json")
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
	if emptyBraceFunction.MatchString(searchText) {
		findings = append(findings, finding(path, firstMatchLine(searchText, emptyBraceFunction), domain.RuleEmptyFunction, domain.SeverityHigh, "empty declaration body"))
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
	return strings.Contains(lower, "panic") || strings.Contains(lower, "throw") || strings.Contains(lower, "raise") || strings.Contains(lower, "fatal")
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
	reader := bufio.NewReader(strings.NewReader(text[:loc[0]]))
	line := 1
	for {
		_, err := reader.ReadString('\n')
		if err != nil {
			return line
		}
		line++
	}
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
