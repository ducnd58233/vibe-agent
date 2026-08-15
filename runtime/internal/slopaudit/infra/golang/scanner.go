package golang

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ducnd58233/vibe-agent/runtime/internal/slopaudit/domain"
)

const (
	DefaultLongFunctionLines = 60
	DefaultWorkers           = 4
	GoFileExtension          = ".go"
)

var skippedDirectories = map[string]struct{}{
	".git":         {},
	".agent-state": {},
	"node_modules": {},
	"vendor":       {},
	"dist":         {},
	"tmp":          {},
}

type Scanner struct {
	workers           int
	longFunctionLines int
}

func NewScanner(workers int) *Scanner {
	if workers <= 0 {
		workers = DefaultWorkers
	}
	return &Scanner{workers: workers, longFunctionLines: DefaultLongFunctionLines}
}

func (s *Scanner) Scan(ctx context.Context, target string) (domain.ScanResult, error) {
	files, err := goFiles(target)
	if err != nil {
		return domain.ScanResult{}, err
	}

	jobs := make(chan string)
	out := make(chan []domain.Finding, len(files))
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
				findings := s.scanFile(path)
				out <- findings
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

	var findings []domain.Finding
	summary := domain.ScanSummary{
		FilesScanned: len(files),
		Languages:    map[string]int{"Go": len(files)},
		Parser:       "go/ast",
	}
	for batch := range out {
		findings = append(findings, batch...)
	}
	for _, path := range files {
		lines, err := countLines(path)
		if err == nil {
			summary.LinesScanned += lines
		}
	}
	return domain.ScanResult{Findings: findings, Summary: summary}, ctx.Err()
}

func countLines(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strings.Count(string(data), "\n") + 1, nil
}

func goFiles(target string) ([]string, error) {
	var files []string
	info, err := os.Stat(target)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		if strings.HasSuffix(target, GoFileExtension) {
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
		if strings.HasSuffix(entry.Name(), GoFileExtension) && !strings.HasSuffix(entry.Name(), "_test.go") {
			files = append(files, filepath.Clean(path))
		}
		return nil
	})
	return files, err
}

func (s *Scanner) scanFile(path string) []domain.Finding {
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, path, nil, parser.ParseComments)
	if err != nil {
		return []domain.Finding{{
			Path:     path,
			Line:     1,
			Rule:     domain.RuleParseError,
			Severity: domain.SeverityHigh,
			Message:  "Go parser could not parse this file",
		}}
	}

	var findings []domain.Finding
	findings = append(findings, commentFindings(set, path, file)...)
	findings = append(findings, s.astFindings(set, path, file)...)
	return findings
}

func commentFindings(set *token.FileSet, path string, file *ast.File) []domain.Finding {
	var findings []domain.Finding
	for _, group := range file.Comments {
		text := strings.ToLower(group.Text())
		if strings.Contains(text, "todo") || strings.Contains(text, "fixme") || strings.Contains(text, "hack") {
			findings = append(findings, domain.Finding{
				Path:     path,
				Line:     set.Position(group.Pos()).Line,
				Rule:     domain.RuleTodoComment,
				Severity: domain.SeverityLow,
				Message:  "unfinished marker in comment",
			})
		}
	}
	return findings
}

func (s *Scanner) astFindings(set *token.FileSet, path string, file *ast.File) []domain.Finding {
	var findings []domain.Finding
	ast.Inspect(file, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.FuncDecl:
			findings = append(findings, s.functionFindings(set, path, n.Name.Name, n.Pos(), n.End(), n.Body)...)
		case *ast.FuncLit:
			findings = append(findings, s.functionFindings(set, path, "function literal", n.Pos(), n.End(), n.Body)...)
		case *ast.AssignStmt:
			if ignoredResult(n) {
				findings = append(findings, domain.Finding{Path: path, Line: set.Position(n.Pos()).Line, Rule: domain.RuleIgnoredResult, Severity: domain.SeverityMedium, Message: "assignment discards a call result"})
			}
		case *ast.CallExpr:
			if isDebugPrint(n) {
				findings = append(findings, domain.Finding{Path: path, Line: set.Position(n.Pos()).Line, Rule: domain.RuleDebugPrint, Severity: domain.SeverityLow, Message: "fmt print call left in runtime code"})
			}
			if isPanicPlaceholder(n) {
				findings = append(findings, domain.Finding{Path: path, Line: set.Position(n.Pos()).Line, Rule: domain.RulePanicPlaceholder, Severity: domain.SeverityHigh, Message: "panic placeholder looks like unfinished code"})
			}
		}
		return true
	})
	return findings
}

func (s *Scanner) functionFindings(set *token.FileSet, path, name string, start, end token.Pos, body *ast.BlockStmt) []domain.Finding {
	if body == nil {
		return nil
	}
	var findings []domain.Finding
	line := set.Position(start).Line
	if len(body.List) == 0 {
		findings = append(findings, domain.Finding{Path: path, Line: line, Rule: domain.RuleEmptyFunction, Severity: domain.SeverityHigh, Message: "function " + name + " has an empty body"})
	}
	lines := set.Position(end).Line - line + 1
	if lines > s.longFunctionLines {
		findings = append(findings, domain.Finding{Path: path, Line: line, Rule: domain.RuleLongFunction, Severity: domain.SeverityMedium, Message: "function " + name + " is longer than the configured limit"})
	}
	return findings
}

func ignoredResult(stmt *ast.AssignStmt) bool {
	for _, left := range stmt.Lhs {
		ident, ok := left.(*ast.Ident)
		if ok && ident.Name == "_" {
			for _, right := range stmt.Rhs {
				if _, ok := right.(*ast.CallExpr); ok {
					return true
				}
			}
		}
	}
	return false
}

func isDebugPrint(call *ast.CallExpr) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := selector.X.(*ast.Ident)
	if !ok || pkg.Name != "fmt" {
		return false
	}
	switch selector.Sel.Name {
	case "Print", "Printf", "Println":
		return hasDebugMarker(call.Args)
	default:
		return false
	}
}

func hasDebugMarker(args []ast.Expr) bool {
	for _, arg := range args {
		lit, ok := arg.(*ast.BasicLit)
		if !ok {
			continue
		}
		text := strings.ToLower(lit.Value)
		if strings.Contains(text, "debug") || strings.Contains(text, "todo") || strings.Contains(text, "temporary") {
			return true
		}
	}
	return false
}

func isPanicPlaceholder(call *ast.CallExpr) bool {
	ident, ok := call.Fun.(*ast.Ident)
	if !ok || ident.Name != "panic" || len(call.Args) == 0 {
		return false
	}
	lit, ok := call.Args[0].(*ast.BasicLit)
	if !ok {
		return false
	}
	text := strings.ToLower(lit.Value)
	return strings.Contains(text, "todo") || strings.Contains(text, "not implemented") || strings.Contains(text, "unimplemented") || strings.Contains(text, "placeholder")
}
