// Package repomap builds a token-budgeted map of the most referenced definitions
// in a workspace. v1 ranks by cross-file reference in-degree only: no PageRank
// and no cache across calls.
package repomap

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/fetch"
)

// Options control one map build.
type Options struct {
	Budget int
	Focus  string
}

// Result is what callers render or return over MCP.
type Result struct {
	Text          string
	Tokens        int
	FilesIncluded int
	FilesOmitted  int
}

// DefaultBudget matches vibe_fetch so the two surfaces cost the same when
// neither caller names a budget.
const DefaultBudget = 4000

// focusBias lifts matching paths above zero-score peers without excluding them.
const focusBias = 1_000

// Build walks root, tags definitions and references, ranks by in-degree, and
// renders the top definitions within budget.
func Build(ctx context.Context, root string, options Options) (Result, error) {
	_ = ctx
	budget := options.Budget
	if budget <= 0 {
		budget = DefaultBudget
	}
	focus := filepath.ToSlash(strings.TrimSpace(options.Focus))

	files, err := listSourceFiles(root)
	if err != nil {
		return Result{}, err
	}

	tagged, err := tagFiles(root, files)
	if err != nil {
		return Result{}, err
	}

	ranked := rankByInDegree(tagged, focus)
	text, included, omitted := render(ranked, budget)
	return Result{
		Text:          text,
		Tokens:        fetch.EstimateTokens(text),
		FilesIncluded: included,
		FilesOmitted:  omitted,
	}, nil
}

type fileTags struct {
	Path string
	Defs []symbol
	Refs []symbol
}

type symbol struct {
	Kind string
	Name string
}

type rankedFile struct {
	Path  string
	Score float64
	Defs  []symbol
}

func rankByInDegree(files []fileTags, focus string) []rankedFile {
	defs := map[string]string{} // symbol name -> defining file (first wins)
	for _, f := range files {
		for _, d := range f.Defs {
			name := strings.TrimSpace(d.Name)
			if name == "" {
				continue
			}
			if _, ok := defs[name]; !ok {
				defs[name] = f.Path
			}
		}
	}

	scores := map[string]float64{}
	defsByPath := map[string][]symbol{}
	for _, f := range files {
		scores[f.Path] = 0
		defsByPath[f.Path] = append([]symbol(nil), f.Defs...)
	}

	for _, f := range files {
		seen := map[string]bool{}
		for _, r := range f.Refs {
			name := strings.TrimSpace(r.Name)
			defPath, ok := defs[name]
			if !ok || defPath == f.Path {
				continue
			}
			if seen[defPath] {
				continue
			}
			seen[defPath] = true
			scores[defPath]++
		}
	}

	out := make([]rankedFile, 0, len(scores))
	for path, score := range scores {
		if focus != "" && strings.HasPrefix(filepath.ToSlash(path), focus) {
			// Additive bias so a zero-score file under focus still rises without
			// dropping every other file from the map.
			score += focusBias
		}
		out = append(out, rankedFile{
			Path:  path,
			Score: score,
			Defs:  defsByPath[path],
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Path < out[j].Path
	})
	return out
}

func render(ranked []rankedFile, budget int) (text string, included, omitted int) {
	var b strings.Builder
	withDefs := 0
	for _, file := range ranked {
		if len(file.Defs) == 0 {
			continue
		}
		withDefs++
		var block strings.Builder
		fmt.Fprintf(&block, "%s:\n", filepath.ToSlash(file.Path))
		for _, d := range file.Defs {
			kind := strings.TrimPrefix(d.Kind, "definition.")
			if kind == "" {
				kind = "symbol"
			}
			fmt.Fprintf(&block, "  %s %s\n", kind, d.Name)
		}
		candidate := b.String() + block.String()
		if fetch.EstimateTokens(candidate) > budget && b.Len() > 0 {
			continue
		}
		if fetch.EstimateTokens(block.String()) > budget && b.Len() == 0 {
			clipped, _ := fetch.Clip(block.String(), budget)
			return clipped, 1, withDefs - 1
		}
		b.WriteString(block.String())
		included++
	}
	omitted = withDefs - included
	return b.String(), included, omitted
}
