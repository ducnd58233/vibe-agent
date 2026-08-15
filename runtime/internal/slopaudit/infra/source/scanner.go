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
	ParserText      = "go-enry language detection plus text rules with optional tree-sitter adapters"
)

var skippedDirectories = map[string]struct{}{
	".git": {},
}

var (
	emptyBraceFunction = regexp.MustCompile(`(?i)\b(func|function|fn|def|class|interface|method)\b[^\n{}]*\{\s*\}`)
	emptyPythonBody    = regexp.MustCompile(`(?i)^\s*(async\s+)?def\s+\w+\([^)]*\):\s*$`)
	ignoredCall        = regexp.MustCompile(`(^|[^[:alnum:]_])_\s*=\s*[[:alnum:]_\.]+\s*\(`)
)

type Scanner struct {
	workers       int
	longFileLines int
}

func NewScanner(workers int) *Scanner {
	if workers <= 0 {
		workers = DefaultWorkers
	}
	return &Scanner{workers: workers, longFileLines: DefaultLongFileLines}
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
		result.Summary.Languages[scanned.language]++
	}
	return result, ctx.Err()
}

type fileResult struct {
	findings []domain.Finding
	language string
	lines    int
	skipped  bool
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
	err = filepath.WalkDir(target, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if filepath.Clean(path) != root {
				if _, ok := skippedDirectories[entry.Name()]; ok {
					return filepath.SkipDir
				}
				if enry.IsVendor(filepath.ToSlash(path)) {
					return filepath.SkipDir
				}
			}
			return nil
		}
		files = append(files, filepath.Clean(path))
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
	data, err := fs.ReadFile(os.DirFS(filepath.Dir(path)), filepath.Base(path))
	if err != nil {
		return fileResult{language: LanguageUnknown, lines: 1, findings: []domain.Finding{{Path: path, Line: 1, Rule: domain.RuleScanError, Severity: domain.SeverityHigh, Message: err.Error()}}}
	}
	if skipFile(path, data) {
		return fileResult{skipped: true}
	}
	language := sourceLanguage(path, data)
	text := string(data)
	lines := strings.Split(text, "\n")
	findings := s.lineFindings(path, language, lines)
	findings = append(findings, fileFindings(path, text, lines)...)
	return fileResult{findings: findings, language: language, lines: len(lines)}
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
	duplicates := map[string]int{}
	for index, line := range lines {
		lineNumber := index + 1
		lower := strings.ToLower(line)
		trimmed := strings.TrimSpace(line)
		if hasUnfinishedMarker(lower) {
			findings = append(findings, finding(path, lineNumber, domain.RuleTodoComment, domain.SeverityLow, "unfinished marker in source text"))
		}
		if ignoredCall.MatchString(line) {
			findings = append(findings, finding(path, lineNumber, domain.RuleIgnoredResult, domain.SeverityMedium, "assignment discards a call result"))
		}
		if hasDebugOutput(language, lower) {
			findings = append(findings, finding(path, lineNumber, domain.RuleDebugPrint, domain.SeverityLow, "debug output call left in source"))
		}
		if hasPlaceholderAbort(lower) {
			findings = append(findings, finding(path, lineNumber, domain.RulePanicPlaceholder, domain.SeverityHigh, "placeholder abort looks like unfinished code"))
		}
		if len(trimmed) >= DuplicateLineMinLength {
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
	if emptyBraceFunction.MatchString(text) {
		findings = append(findings, finding(path, firstMatchLine(text, emptyBraceFunction), domain.RuleEmptyFunction, domain.SeverityHigh, "empty declaration body"))
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

func hasUnfinishedMarker(lower string) bool {
	return strings.Contains(lower, "todo") || strings.Contains(lower, "fixme") || strings.Contains(lower, "hack") || strings.Contains(lower, "placeholder")
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
	if !strings.Contains(lower, "todo") && !strings.Contains(lower, "not implemented") && !strings.Contains(lower, "unimplemented") && !strings.Contains(lower, "placeholder") {
		return false
	}
	return strings.Contains(lower, "panic") || strings.Contains(lower, "throw") || strings.Contains(lower, "raise") || strings.Contains(lower, "fatal")
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

func finding(path string, line int, rule string, severity domain.Severity, message string) domain.Finding {
	return domain.Finding{Path: path, Line: line, Rule: rule, Severity: severity, Message: message}
}
