// Package reviewscan does the mechanical half of a code review so an agent
// does not have to: it reads every source file in a workspace (installed
// packages, build output, and generated files excluded), cuts each file into
// blocks that together cover every line, and reports what can be decided from
// the syntax tree alone: unused imports, definitions nothing references, and
// bug shapes such as self-comparisons, identical branches, unreachable
// statements, duplicate keys, and swallowed errors.
//
// It is advisory. Findings are leads for a reviewer, never gate evidence:
// nothing here resolves types, so a name used only through reflection or a
// framework looks unreferenced, and the report says so.
//
// Language knowledge is data. Rules match node shapes that read the same in
// every tree-sitter grammar (a binary node whose operands have the same text,
// an if whose consequence equals its alternative), and the few facts that do
// differ per language (how an import binds a name, how a definition is made
// private) live in the spec table in lang.go.
package reviewscan

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/ducnd58233/vibe-agent/runtime/internal/sourcefiles"
)

// Severity orders findings. High is very likely a defect; info is context a
// reviewer should see but not necessarily act on.
type Severity string

const (
	SeverityHigh   Severity = "high"
	SeverityMedium Severity = "medium"
	SeverityLow    Severity = "low"
	SeverityInfo   Severity = "info"
)

// ParseSeverity accepts a severity name.
func ParseSeverity(value string) (Severity, error) {
	switch s := Severity(strings.ToLower(strings.TrimSpace(value))); s {
	case SeverityHigh, SeverityMedium, SeverityLow, SeverityInfo:
		return s, nil
	}
	return "", fmt.Errorf("unknown severity %q; use high, medium, low, or info", value)
}

func (s Severity) rank() int {
	switch s {
	case SeverityHigh:
		return 3
	case SeverityMedium:
		return 2
	case SeverityLow:
		return 1
	}
	return 0
}

// AtLeast reports whether s is as severe as floor.
func (s Severity) AtLeast(floor Severity) bool { return s.rank() >= floor.rank() }

// Finding is one lead, anchored to a line and the innermost block holding it.
type Finding struct {
	Path     string   `json:"path"`
	Line     int      `json:"line"`
	Rule     string   `json:"rule"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	Block    string   `json:"block,omitempty"`
	Code     string   `json:"code,omitempty"`
}

// Block is a contiguous range of one file to read as a unit: a definition,
// or a stretch of top-level code between definitions. A file's blocks cover
// every non-blank line.
type Block struct {
	Kind     string `json:"kind"`
	Name     string `json:"name,omitempty"`
	Start    int    `json:"start"`
	End      int    `json:"end"`
	Depth    int    `json:"depth"`
	Refs     *int   `json:"refs,omitempty"`
	Changed  bool   `json:"changed,omitempty"`
	Findings int    `json:"findings"`
}

// Label names a block the way findings and the worklist print it.
func (b Block) Label() string {
	if b.Name == "" {
		return fmt.Sprintf("%s L%d-L%d", b.Kind, b.Start, b.End)
	}
	return fmt.Sprintf("%s %s L%d-L%d", b.Kind, b.Name, b.Start, b.End)
}

// File is one in-scope source file and its blocks.
type File struct {
	Path     string `json:"path"`
	Language string `json:"language"`
	Lines    int    `json:"lines"`
	Parsed   bool   `json:"parsed"`
	// SyntaxError is the first line the grammar could not parse; block
	// boundaries after it are approximate.
	SyntaxError int     `json:"syntax_error,omitempty"`
	Test        bool    `json:"test,omitempty"`
	Changed     bool    `json:"changed,omitempty"`
	Blocks      []Block `json:"blocks"`
}

// Summary counts what the scan covered.
type Summary struct {
	Files     int              `json:"files"`
	Indexed   int              `json:"indexed"`
	Blocks    int              `json:"blocks"`
	Unparsed  []string         `json:"unparsed,omitempty"`
	Findings  map[Severity]int `json:"findings"`
	Languages map[string]int   `json:"languages"`
}

// Report is the result of one scan.
type Report struct {
	Root     string    `json:"root"`
	Base     string    `json:"base,omitempty"`
	Summary  Summary   `json:"summary"`
	Findings []Finding `json:"findings"`
	Files    []File    `json:"files"`

	index *index
}

// Options narrow what a scan reports. The reference index always covers the
// whole workspace, so a definition used only outside the narrowed paths is
// still counted as used.
type Options struct {
	// Paths, relative to the root, limit reporting to files under them.
	Paths []string
	// Changed limits reporting to files changed since Base (a merge base is
	// found when Base is empty), and marks the blocks whose lines changed.
	Changed bool
	Base    string
	// MinSeverity drops findings below it. Empty means low.
	MinSeverity Severity
	Workers     int
}

// Scan reads every source file under root and reports the in-scope ones.
func Scan(ctx context.Context, root string, options Options) (Report, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return Report{}, err
	}
	floor := options.MinSeverity
	if floor == "" {
		floor = SeverityLow
	}
	scope, err := newScope(abs, options.Paths)
	if err != nil {
		return Report{}, err
	}
	var diff *changes
	if options.Changed {
		if diff, err = changedSince(ctx, abs, options.Base); err != nil {
			return Report{}, err
		}
	}

	listed, err := sourcefiles.List(ctx, abs)
	if err != nil {
		return Report{}, err
	}
	analyses := analyzeAll(ctx, listed, options.Workers)
	if err := ctx.Err(); err != nil {
		return Report{}, err
	}

	idx := buildIndex(analyses)
	idx.crossFileFindings(diff)

	report := Report{Root: abs, index: idx, Summary: Summary{Findings: map[Severity]int{}, Languages: map[string]int{}}}
	if diff != nil {
		report.Base = diff.base
	}
	for _, a := range analyses {
		if a == nil {
			continue
		}
		report.Summary.Indexed++
		if !a.code || !scope.has(a.file.Path) {
			continue
		}
		file := a.file
		changed := diff != nil && diff.touched(file.Path)
		orphans := diff != nil && a.hasOrphanFinding()
		if diff != nil && !changed && !orphans {
			continue
		}
		file.Changed = changed
		for i := range file.Blocks {
			if diff != nil {
				file.Blocks[i].Changed = diff.overlaps(file.Path, file.Blocks[i].Start, file.Blocks[i].End)
			}
		}
		for _, finding := range a.findings {
			if !finding.Severity.AtLeast(floor) || a.suppressed(finding.Line) {
				continue
			}
			block := innermost(file.Blocks, finding.Line)
			if diff != nil && !diff.keeps(file, block, finding) {
				continue
			}
			if block >= 0 {
				file.Blocks[block].Findings++
				finding.Block = file.Blocks[block].Label()
			}
			report.Findings = append(report.Findings, finding)
			report.Summary.Findings[finding.Severity]++
		}
		if !a.parsed {
			report.Summary.Unparsed = append(report.Summary.Unparsed, file.Path)
		}
		report.Summary.Files++
		report.Summary.Blocks += len(file.Blocks)
		report.Summary.Languages[file.Language]++
		report.Files = append(report.Files, file)
	}
	sort.SliceStable(report.Findings, func(i, j int) bool {
		a, b := report.Findings[i], report.Findings[j]
		if a.Severity.rank() != b.Severity.rank() {
			return a.Severity.rank() > b.Severity.rank()
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.Line < b.Line
	})
	return report, nil
}

func analyzeAll(ctx context.Context, listed []sourcefiles.File, workers int) []*analysis {
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	out := make([]*analysis, len(listed))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			parsers := parserCache{}
			for i := range jobs {
				data, err := os.ReadFile(listed[i].Abs)
				if err != nil || !sourcefiles.Readable(listed[i].Rel, data) {
					continue
				}
				out[i] = analyze(listed[i].Rel, data, parsers)
			}
		}()
	}
	for i := range listed {
		if ctx.Err() != nil {
			break
		}
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return out
}

// innermost returns the index of the deepest block holding line, or -1.
func innermost(blocks []Block, line int) int {
	best := -1
	for i, b := range blocks {
		if line < b.Start || line > b.End {
			continue
		}
		if best < 0 || b.Depth > blocks[best].Depth || (b.Depth == blocks[best].Depth && b.Kind != kindTopLevel) {
			best = i
		}
	}
	return best
}

// scope is the set of paths a scan reports on.
type scope struct{ prefixes []string }

func newScope(root string, paths []string) (scope, error) {
	var s scope
	for _, path := range paths {
		abs, err := filepath.Abs(path)
		if err != nil {
			return s, err
		}
		rel, err := filepath.Rel(root, abs)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return s, fmt.Errorf("%s is outside the scan root %s", path, root)
		}
		if _, err := os.Stat(abs); err != nil {
			return s, err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return scope{}, nil
		}
		s.prefixes = append(s.prefixes, rel)
	}
	return s, nil
}

func (s scope) has(path string) bool {
	if len(s.prefixes) == 0 {
		return true
	}
	for _, prefix := range s.prefixes {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}
