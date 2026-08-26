package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ClaudeIfOutsideClaude reports Claude-style handler "if" keys found in
// non-Claude host hook configs under toolkitRoot.
//
// Claude settings may use "if" (permission-rule syntax). Cursor and Codex
// shipped configs must not copy that field: those hosts do not honor it, and
// a silent no-op would look like portable filtering.
func ClaudeIfOutsideClaude(toolkitRoot string) []string {
	var problems []string
	for _, rel := range []string{
		filepath.Join(".cursor", "hooks.json"),
		filepath.Join(".codex", "hooks.json"),
	} {
		path := filepath.Join(toolkitRoot, rel)
		raw, err := os.ReadFile(filepath.Clean(path))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", rel, err))
			continue
		}
		for _, hit := range findHandlerIfKeys(raw) {
			problems = append(problems, fmt.Sprintf("%s: Claude-style hook if %q (Claude-only; use host matcher instead)", rel, hit))
		}
	}
	return problems
}

func findHandlerIfKeys(raw []byte) []string {
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil
	}
	hooks, _ := root["hooks"].(map[string]any)
	if hooks == nil {
		return nil
	}
	var hits []string
	for _, eventVal := range hooks {
		blocks, ok := eventVal.([]any)
		if !ok {
			continue
		}
		for _, block := range blocks {
			obj, ok := block.(map[string]any)
			if !ok {
				continue
			}
			// Cursor flat: { "command", "matcher", "if"? }
			if ifVal, has := obj["if"]; has {
				hits = append(hits, fmt.Sprint(ifVal))
			}
			// Claude/Codex nested: { "matcher", "hooks": [ { "if", "command" } ] }
			inners, _ := obj["hooks"].([]any)
			for _, inner := range inners {
				innerObj, ok := inner.(map[string]any)
				if !ok {
					continue
				}
				if ifVal, has := innerObj["if"]; has {
					hits = append(hits, fmt.Sprint(ifVal))
				}
			}
		}
	}
	return hits
}

// FormatClaudeIfProblems joins ClaudeIfOutsideClaude hits for test failure text.
func FormatClaudeIfProblems(problems []string) string {
	return strings.Join(problems, "; ")
}
