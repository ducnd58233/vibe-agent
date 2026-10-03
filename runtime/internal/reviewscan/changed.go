package reviewscan

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/safexec"
)

// changes is what a diff against a base touched, relative to the scan root.
type changes struct {
	base    string
	ranges  map[string][][2]int
	whole   map[string]bool
	removed map[string]bool
}

// baseCandidates are tried in order when no base is named: the branch's
// upstream, then the remote's default branch, then common local names.
var baseCandidates = []string{"@{upstream}", "origin/HEAD", "origin/main", "origin/master", "main", "master"}

func git(ctx context.Context, root string, args ...string) ([]byte, error) {
	cmd, err := safexec.CommandContext(ctx, "git", append([]string{"-C", root, "-c", "core.quotePath=false"}, args...)...)
	if err != nil {
		return nil, err
	}
	return cmd.Output()
}

func changedSince(ctx context.Context, root, base string) (*changes, error) {
	candidates := baseCandidates
	if base != "" {
		candidates = []string{base}
	}
	var fork string
	for _, candidate := range candidates {
		out, err := git(ctx, root, "merge-base", "HEAD", candidate)
		if err == nil {
			fork, base = strings.TrimSpace(string(out)), candidate
			break
		}
	}
	if fork == "" {
		return nil, fmt.Errorf("no merge base with %s; pass --base <ref>", strings.Join(candidates, ", "))
	}
	diff, err := git(ctx, root, "diff", "--relative", "--no-color", "--no-ext-diff", "-U0", fork, "--", ".")
	if err != nil {
		return nil, fmt.Errorf("git diff %s: %w", base, err)
	}
	c := parseDiff(diff)
	c.base = base
	untracked, err := git(ctx, root, "ls-files", "-z", "--others", "--exclude-standard")
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}
	for _, raw := range bytes.Split(untracked, []byte{0}) {
		if path := string(raw); path != "" {
			c.whole[path] = true
		}
	}
	return c, nil
}

var hunkHeader = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)

// parseDiff reads a zero-context unified diff: the new-side line ranges per
// file, and every word on a removed line (a name whose last use was deleted).
func parseDiff(diff []byte) *changes {
	c := &changes{ranges: map[string][][2]int{}, whole: map[string]bool{}, removed: map[string]bool{}}
	current := ""
	scanner := bufio.NewScanner(bytes.NewReader(diff))
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		text := scanner.Text()
		switch {
		case strings.HasPrefix(text, "+++ "):
			current = strings.TrimPrefix(strings.TrimPrefix(text, "+++ "), "b/")
			if current == "/dev/null" {
				current = ""
			}
		case strings.HasPrefix(text, "--- "):
		case strings.HasPrefix(text, "@@"):
			m := hunkHeader.FindStringSubmatch(text)
			if m == nil || current == "" {
				continue
			}
			start, _ := strconv.Atoi(m[1])
			count := 1
			if m[2] != "" {
				count, _ = strconv.Atoi(m[2])
			}
			end := start + count - 1
			if count == 0 { // a pure deletion after line start
				end = start + 1
			}
			c.ranges[current] = append(c.ranges[current], [2]int{max(start, 1), end})
		case strings.HasPrefix(text, "-"):
			addWords(c.removed, text[1:])
		}
	}
	return c
}

func (c *changes) touched(path string) bool {
	return c.whole[path] || len(c.ranges[path]) > 0
}

func (c *changes) overlaps(path string, start, end int) bool {
	if c.whole[path] {
		return true
	}
	for _, r := range c.ranges[path] {
		if r[0] <= end && start <= r[1] {
			return true
		}
	}
	return false
}

// keeps decides whether a finding belongs to the change: on a changed line,
// inside a changed definition, an import of a changed file (an edit elsewhere
// can remove its last use), or a definition the change left without callers.
func (c *changes) keeps(file File, block int, f Finding) bool {
	switch {
	case f.Rule == ruleOrphaned, c.overlaps(file.Path, f.Line, f.Line):
		return true
	case f.Rule == ruleUnusedImport:
		return c.touched(file.Path)
	case block >= 0 && file.Blocks[block].Kind != kindTopLevel:
		return file.Blocks[block].Changed
	}
	return false
}
