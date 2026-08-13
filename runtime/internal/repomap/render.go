package repomap

import (
	"fmt"
	"sort"
	"strings"
)

// charsPerToken is the ratio this package budgets by.
//
// An estimate, and named as one. Counting real tokens means shipping a
// tokenizer for a model whose vocabulary changes, to decide how much of a
// summary to print. Four characters per token is the usual approximation for
// English and code, and the consequence of it being off is a map slightly
// larger or smaller than asked for, not a wrong map.
const charsPerToken = 4

func estimateTokens(text string) int {
	return (len(text) + charsPerToken - 1) / charsPerToken
}

// Render writes the map as text, within a token budget.
//
// Files come out in rank order, grouped under their directory. When the budget
// binds, the remainder is reported as a count rather than dropped in silence: an
// agent that knows 200 files were omitted asks for a bigger budget, and one that
// does not assumes it has seen the repository.
func Render(result Result, budget int) string {
	var out strings.Builder
	// Reserve room for the footer, which has to fit whatever else happens.
	reserve := 24
	var written int

	// Group by directory while keeping the rank order of first appearance, so
	// the most-referenced directory leads.
	type group struct {
		dir   string
		files []File
	}
	var groups []group
	seen := map[string]int{}
	for _, file := range result.Files {
		dir := directoryOf(file.Path)
		index, ok := seen[dir]
		if !ok {
			seen[dir] = len(groups)
			groups = append(groups, group{dir: dir})
			index = len(groups) - 1
		}
		groups[index].files = append(groups[index].files, file)
	}

	for _, g := range groups {
		header := g.dir + "\n"
		if estimateTokens(out.String())+estimateTokens(header) > budget-reserve {
			break
		}
		var body strings.Builder
		body.WriteString(header)
		var kept int
		for _, file := range g.files {
			line := "  " + fileLine(file) + "\n"
			if estimateTokens(out.String())+estimateTokens(body.String())+
				estimateTokens(line) > budget-reserve {
				break
			}
			body.WriteString(line)
			kept++
		}
		if kept == 0 {
			break
		}
		out.WriteString(body.String())
		written += kept
	}

	if omitted := len(result.Files) - written; omitted > 0 {
		fmt.Fprintf(&out, "\n%d files omitted; raise the budget to see them\n", omitted)
	}
	fmt.Fprintf(&out, "\n%d files: %d read, %d from cache\n",
		len(result.Files), result.Read, result.Cached)
	return out.String()
}

// fileLine is one file's row: its name, then what it declares.
func fileLine(file File) string {
	name := file.Path
	if slash := strings.LastIndexByte(name, '/'); slash >= 0 {
		name = name[slash+1:]
	}
	if len(file.Symbols) == 0 {
		return name
	}
	names := make([]string, 0, len(file.Symbols))
	for _, symbol := range file.Symbols {
		names = append(names, symbol.Name)
	}
	return name + "  " + strings.Join(names, ", ")
}

func directoryOf(path string) string {
	if slash := strings.LastIndexByte(path, '/'); slash >= 0 {
		return path[:slash+1]
	}
	return "./"
}

// Sorted returns the map ordered by path rather than by rank, for a reader who
// wants the tree rather than the ranking.
func Sorted(result Result) []File {
	files := make([]File, len(result.Files))
	copy(files, result.Files)
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files
}
