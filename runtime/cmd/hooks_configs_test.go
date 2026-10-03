package main

import (
	"path/filepath"
	"testing"
)

// hookConfigs is derived from the contract table; this pins the one fact that
// used to be a second list: a host's config file, including a host with two.
func TestHookConfigsCoverEveryContractPath(t *testing.T) {
	have := map[string]bool{}
	for _, config := range hookConfigs {
		have[config.Path] = true
	}
	for _, want := range []string{".codex/config.toml", ".kimi/hooks.toml", "opencode.json"} {
		if !have[filepath.FromSlash(want)] {
			t.Errorf("hook configs lack %s", want)
		}
	}
}
