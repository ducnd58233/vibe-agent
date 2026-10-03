package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// UnsupportedHandlerIf reports handler "if" keys found in the hook configs of
// hosts whose contract says they do not honor one.
//
// An "if" is permission-rule syntax some hosts filter handlers with. A config
// for a host that ignores it must not copy the field: a silent no-op would look
// like portable filtering. Only JSON configs are inspected, and a file that does
// not exist is skipped, since a consumer repo need not wire every host.
func UnsupportedHandlerIf(toolkitRoot string) []string {
	var problems []string
	for _, contract := range hostContracts {
		if contract.HonorsHandlerIf || filepath.Ext(contract.ConfigPath) != ".json" {
			continue
		}
		rel := filepath.FromSlash(contract.ConfigPath)
		raw, err := os.ReadFile(filepath.Clean(filepath.Join(toolkitRoot, rel)))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", rel, err))
			continue
		}
		for _, hit := range findHandlerIfKeys(raw) {
			problems = append(problems, fmt.Sprintf("%s: hook if %q is not honored by %s; use the host's own matcher instead", rel, hit, contract.Client))
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
			// Flat: { "command", "matcher", "if"? }
			if ifVal, has := obj["if"]; has {
				hits = append(hits, fmt.Sprint(ifVal))
			}
			// Nested: { "matcher", "hooks": [ { "if", "command" } ] }
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

// FormatHandlerIfProblems joins UnsupportedHandlerIf hits for failure text.
func FormatHandlerIfProblems(problems []string) string {
	return strings.Join(problems, "; ")
}
