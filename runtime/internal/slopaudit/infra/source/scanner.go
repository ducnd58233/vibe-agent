package source

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

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
	ParserText      = "text-rules with optional tree-sitter adapters"
)

var skippedDirectories = map[string]struct{}{
	".git":         {},
	".agent-state": {},
	"node_modules": {},
	"vendor":       {},
	"dist":         {},
	"tmp":          {},
	"bin":          {},
}

var skippedFileNames = map[string]struct{}{
	".env":                {},
	".env.local":          {},
	"settings.local.json": {},
}

var languageByName = map[string]string{
	"Dockerfile":       "Dockerfile",
	"Containerfile":    "Dockerfile",
	"Makefile":         "Make",
	"makefile":         "Make",
	"Rakefile":         "Ruby",
	"Gemfile":          "Ruby",
	"Procfile":         "Procfile",
	"docker-compose":   "Yaml",
	"docker-compose.y": "Yaml",
}

var languageByExtension = map[string]string{
	".bash": "Bash", ".bats": "Bash", ".sh": "Bash", ".zsh": "Bash",
	".c": "C", ".h": "C",
	".cc": "Cpp", ".cpp": "Cpp", ".cxx": "Cpp", ".hpp": "Cpp", ".hh": "Cpp",
	".cs":  "CSharp",
	".css": "Css", ".scss": "Scss", ".sass": "Sass", ".less": "Less",
	".ex": "Elixir", ".exs": "Elixir",
	".go":   "Go",
	".hs":   "Haskell",
	".html": "Html", ".htm": "Html", ".xhtml": "Html",
	".java": "Java",
	".js":   "JavaScript", ".jsx": "JavaScript", ".mjs": "JavaScript", ".cjs": "JavaScript",
	".json": "Json",
	".kt":   "Kotlin", ".kts": "Kotlin",
	".lua": "Lua",
	".nix": "Nix",
	".php": "Php",
	".py":  "Python", ".pyi": "Python",
	".rb": "Ruby", ".gemspec": "Ruby",
	".rs":    "Rust",
	".scala": "Scala", ".sc": "Scala", ".sbt": "Scala",
	".sol":   "Solidity",
	".swift": "Swift",
	".ts":    "TypeScript", ".tsx": "Tsx", ".cts": "TypeScript", ".mts": "TypeScript",
	".vue": "Vue", ".svelte": "Svelte", ".astro": "Astro",
	".mdx": "Mdx", ".graphql": "GraphQL", ".gql": "GraphQL",
	".sql": "Sql", ".prisma": "Prisma", ".proto": "Proto",
	".toml": "Toml", ".ini": "Ini",
	".yaml": "Yaml", ".yml": "Yaml",
}

var binaryExtensions = map[string]struct{}{
	".7z": {}, ".a": {}, ".avi": {}, ".bin": {}, ".bmp": {}, ".class": {}, ".dll": {},
	".dmg": {}, ".exe": {}, ".gif": {}, ".gz": {}, ".ico": {}, ".jar": {}, ".jpeg": {},
	".jpg": {}, ".mov": {}, ".mp3": {}, ".mp4": {}, ".o": {}, ".obj": {}, ".pdf": {},
	".png": {}, ".so": {}, ".tar": {}, ".wasm": {}, ".webp": {}, ".zip": {},
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
}

func sourceFiles(target string) ([]string, error) {
	var files []string
	info, err := os.Stat(target)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		if sourceCandidate(target) {
			return []string{filepath.Clean(target)}, nil
		}
		return files, nil
	}
	err = filepath.WalkDir(target, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if _, ok := skippedDirectories[entry.Name()]; ok {
				return filepath.SkipDir
			}
			return nil
		}
		if sourceCandidate(path) {
			files = append(files, filepath.Clean(path))
		}
		return nil
	})
	return files, err
}

func sourceLanguage(path string) string {
	name := filepath.Base(path)
	if language, ok := languageByName[name]; ok {
		return language
	}
	language, ok := languageByExtension[strings.ToLower(filepath.Ext(path))]
	if !ok {
		return LanguageText
	}
	return language
}

func sourceCandidate(path string) bool {
	if _, ok := skippedFileNames[filepath.Base(path)]; ok {
		return false
	}
	if _, ok := binaryExtensions[strings.ToLower(filepath.Ext(path))]; ok {
		return false
	}
	if sourceLanguage(path) != LanguageText {
		return true
	}
	return looksLikeText(path)
}

func looksLikeText(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = file.Close() }()

	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && n == 0 {
		return false
	}
	return !bytes.Contains(buffer[:n], []byte{0})
}

func (s *Scanner) scanFile(path string) fileResult {
	language := sourceLanguage(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return fileResult{language: language, lines: 1, findings: []domain.Finding{{Path: path, Line: 1, Rule: domain.RuleScanError, Severity: domain.SeverityHigh, Message: err.Error()}}}
	}
	if len(data) > MaxFileBytes {
		return fileResult{language: language, lines: 1}
	}
	text := string(data)
	lines := strings.Split(text, "\n")
	findings := s.lineFindings(path, language, lines)
	findings = append(findings, fileFindings(path, text, lines)...)
	return fileResult{findings: findings, language: language, lines: len(lines)}
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
