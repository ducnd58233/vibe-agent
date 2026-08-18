package source

import (
	"os"
	"path/filepath"
	"strings"
)

// gitignore holds patterns from a workspace .gitignore for slop walks.
//
// The matcher is intentionally small: it covers the paths this toolkit
// gitignores (root-anchored dirs, simple globs) without implementing the full
// gitignore spec. Slop only needs to skip evidence and generated views that
// must never affect the score.
type gitignore struct {
	root     string
	patterns []pattern
}

type pattern struct {
	raw      string
	anchored bool
	dirOnly  bool
}

func loadGitignore(root string) gitignore {
	root = filepath.Clean(root)
	if absRoot, err := filepath.Abs(root); err == nil {
		root = absRoot
	}
	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		return gitignore{root: root}
	}
	var patterns []pattern
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		p := pattern{raw: line}
		if strings.HasPrefix(line, "/") {
			p.anchored = true
			line = strings.TrimPrefix(line, "/")
		}
		if strings.HasSuffix(line, "/") {
			p.dirOnly = true
			line = strings.TrimSuffix(line, "/")
		}
		p.raw = line
		patterns = append(patterns, p)
	}
	return gitignore{root: root, patterns: patterns}
}

func (g gitignore) relPath(absPath string) (string, bool) {
	clean := filepath.Clean(absPath)
	if abs, err := filepath.Abs(clean); err == nil {
		clean = abs
	}
	rel, err := filepath.Rel(g.root, clean)
	if err != nil {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

func (g gitignore) skipDir(absPath string) bool {
	rel, ok := g.relPath(absPath)
	if !ok || rel == "." {
		return false
	}
	for _, p := range g.patterns {
		if p.matches(rel, true) {
			return true
		}
	}
	return false
}

func (g gitignore) skipFile(absPath string) bool {
	rel, ok := g.relPath(absPath)
	if !ok {
		return false
	}
	for _, p := range g.patterns {
		if p.matches(rel, false) {
			return true
		}
	}
	return false
}

func (p pattern) matches(rel string, isDir bool) bool {
	if p.dirOnly && !isDir && !strings.HasSuffix(rel, "/") {
		// A dir-only pattern also skips files inside that dir when the walk
		// asks about a directory path; file paths are matched by prefix.
		if !isDir {
			prefix := p.raw + "/"
			if strings.HasPrefix(rel, prefix) {
				return true
			}
		}
	}
	target := p.raw
	if p.anchored {
		if target == rel {
			return true
		}
		if p.dirOnly || strings.Contains(target, "/") {
			prefix := target + "/"
			return strings.HasPrefix(rel, prefix)
		}
		return false
	}
	if strings.Contains(target, "*") {
		return matchSimpleGlob(target, rel, isDir)
	}
	if rel == target {
		return true
	}
	prefix := target + "/"
	return strings.HasPrefix(rel, prefix)
}

func matchSimpleGlob(pattern, rel string, isDir bool) bool {
	// Supports **/middle and trailing * for the slop audit use case only.
	if strings.HasPrefix(pattern, "**/") {
		suffix := strings.TrimPrefix(pattern, "**/")
		if strings.HasSuffix(suffix, "/*") {
			prefix := strings.TrimSuffix(suffix, "/*")
			if idx := strings.Index(rel, "/"+prefix+"/"); idx >= 0 {
				return true
			}
			return strings.HasPrefix(rel, prefix+"/")
		}
		return strings.Contains(rel, "/"+suffix) || strings.HasSuffix(rel, "/"+suffix) || rel == suffix
	}
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(rel, prefix)
	}
	_ = isDir
	return rel == pattern
}
