package reviewscan

import (
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
)

// maxReferencingFiles bounds the caller list a block view prints.
const maxReferencingFiles = 10

// WriteText prints a report as a worklist: findings first, then every block of
// every file in reading order. findingsOnly drops the block list.
func WriteText(out io.Writer, r Report, findingsOnly bool) error {
	p := &printer{w: out}
	base := ""
	if r.Base != "" {
		base = "  changes since " + r.Base
	}
	p.f("review scan: %s%s\n", r.Root, base)
	p.f("  files %d (of %d indexed for references)  blocks %d\n", r.Summary.Files, r.Summary.Indexed, r.Summary.Blocks)
	var counts []string
	for _, s := range []Severity{SeverityHigh, SeverityMedium, SeverityLow, SeverityInfo} {
		if n := r.Summary.Findings[s]; n > 0 {
			counts = append(counts, fmt.Sprintf("%d %s", n, s))
		}
	}
	if len(counts) == 0 {
		counts = []string{"none"}
	}
	p.f("  findings %s\n", strings.Join(counts, ", "))
	if len(r.Summary.Unparsed) > 0 {
		p.f("  no grammar, read whole: %s\n", strings.Join(r.Summary.Unparsed, ", "))
	}

	if len(r.Findings) > 0 {
		p.f("\nFINDINGS (verify each in its block before acting; a scan cannot see reflection, framework, or cross-repo callers)" + "\n")
		for _, f := range r.Findings {
			p.f("  %-6s %s:%d  %s  %s\n", f.Severity, f.Path, f.Line, f.Rule, f.Message)
			if f.Code != "" {
				p.f("         > %s\n", f.Code)
			}
			if f.Block != "" {
				p.f("         in %s\n", f.Block)
			}
		}
	}
	if findingsOnly || len(r.Files) == 0 {
		return p.err
	}
	p.f("\nBLOCKS (review every block in order; `vibe-agent review block <path>:<line>` prints one with its findings and callers)" + "\n")
	for _, file := range r.Files {
		var tags []string
		if file.Test {
			tags = append(tags, "test")
		}
		if file.Changed {
			tags = append(tags, "changed")
		}
		if !file.Parsed {
			tags = append(tags, "not parsed")
		}
		if file.SyntaxError > 0 {
			tags = append(tags, fmt.Sprintf("syntax error near L%d, block bounds approximate", file.SyntaxError))
		}
		extra := ""
		if len(tags) > 0 {
			extra = ", " + strings.Join(tags, ", ")
		}
		p.f("%s (%s, %d lines%s)\n", file.Path, file.Language, file.Lines, extra)
		for _, b := range file.Blocks {
			mark := " "
			if b.Changed {
				mark = "*"
			}
			label := b.Kind
			if b.Name != "" {
				label += " " + b.Name
			}
			detail := ""
			if b.Refs != nil {
				detail += fmt.Sprintf("  refs %d", *b.Refs)
			}
			if b.Findings > 0 {
				detail += fmt.Sprintf("  %d finding(s)", b.Findings)
			}
			p.f(" %s %s%-12s %s%s\n", mark, strings.Repeat("  ", b.Depth), fmt.Sprintf("L%d-L%d", b.Start, b.End), label, detail)
		}
	}
	return p.err
}

// printer writes formatted text and keeps the first write error, so a render
// reports a closed pipe once instead of checking every line.
type printer struct {
	w   io.Writer
	err error
}

func (p *printer) f(format string, args ...any) {
	if p.err == nil {
		_, p.err = fmt.Fprintf(p.w, format, args...)
	}
}

// BlockView is one block with its source, its findings, and who uses it.
type BlockView struct {
	File         File      `json:"file"`
	Block        Block     `json:"block"`
	Source       []string  `json:"source"`
	Nested       []Block   `json:"nested,omitempty"`
	Findings     []Finding `json:"findings,omitempty"`
	ReferencedIn []string  `json:"referenced_in,omitempty"`
}

// Block finds a block by "path:line" or "path:name" in a report.
func (r Report) Block(target string) (BlockView, error) {
	cut := strings.LastIndexByte(target, ':')
	if cut <= 0 || cut == len(target)-1 {
		return BlockView{}, fmt.Errorf("want <path>:<line> or <path>:<name>, got %q", target)
	}
	path, where := r.relPath(target[:cut]), target[cut+1:]
	var file *File
	for i := range r.Files {
		if r.Files[i].Path == path {
			file = &r.Files[i]
		}
	}
	if file == nil {
		return BlockView{}, fmt.Errorf("%s is not a scanned source file (skipped as vendored, generated, binary, or not code)", path)
	}
	index := -1
	if lineNo, err := strconv.Atoi(where); err == nil {
		index = innermost(file.Blocks, lineNo)
	} else {
		for i, b := range file.Blocks {
			if b.Name == where || strings.HasSuffix(b.Name, "."+where) {
				index = i
				break
			}
		}
	}
	if index < 0 {
		return BlockView{}, fmt.Errorf("no block at %s in %s", where, path)
	}
	block := file.Blocks[index]
	view := BlockView{File: *file, Block: block}
	view.File.Blocks = nil
	for _, a := range r.index.analyses {
		if a != nil && a.file.Path == path {
			view.Source = a.lines[block.Start-1 : min(block.End, len(a.lines))]
		}
	}
	for _, b := range file.Blocks {
		if b.Depth == block.Depth+1 && b.Start >= block.Start && b.End <= block.End && b.Kind != kindTopLevel {
			view.Nested = append(view.Nested, b)
		}
	}
	for _, f := range r.Findings {
		if f.Path == path && f.Block == block.Label() {
			view.Findings = append(view.Findings, f)
		}
	}
	if block.Name != "" {
		name := block.Name
		if dot := strings.LastIndexByte(name, '.'); dot >= 0 {
			name = name[dot+1:]
		}
		view.ReferencedIn = r.ReferencedIn(name, maxReferencingFiles)
	}
	return view, nil
}

func (r Report) relPath(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		if rel, err := filepath.Rel(r.Root, abs); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(filepath.Clean(path))
}

// WriteBlock prints a block's source with line numbers, folding nested blocks
// to one line each so a long class reads as its own lines plus a list of its
// members.
func WriteBlock(out io.Writer, view BlockView) error {
	p := &printer{w: out}
	b := view.Block
	p.f("%s  %s\n", view.File.Path, b.Label())
	if b.Refs != nil {
		callers := ""
		if len(view.ReferencedIn) > 0 {
			callers = " in " + strings.Join(view.ReferencedIn, ", ")
		}
		p.f("references: %d%s\n", *b.Refs, callers)
	}
	if len(view.Findings) > 0 {
		p.f("findings:" + "\n")
		for _, f := range view.Findings {
			p.f("  L%d %s %s: %s\n", f.Line, f.Severity, f.Rule, f.Message)
		}
	}
	p.f("source:" + "\n")
	width := len(strconv.Itoa(b.End))
	for i := 0; i < len(view.Source); i++ {
		lineNo := b.Start + i
		if nested := nestedAt(view.Nested, lineNo); nested != nil && lineNo != b.Start {
			p.f("%*s | ... %s (folded; print it with `vibe-agent review block %s:%d`)\n", width, "", nested.Label(), view.File.Path, nested.Start)
			i += nested.End - lineNo
			continue
		}
		p.f("%*d | %s\n", width, lineNo, view.Source[i])
	}
	return p.err
}

func nestedAt(nested []Block, lineNo int) *Block {
	for i := range nested {
		if nested[i].Start == lineNo {
			return &nested[i]
		}
	}
	return nil
}
