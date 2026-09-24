package docmeta_test

import (
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/docmeta"
)

func TestLooksTransliteratedFlagsBareConsonantFragments(t *testing.T) {
	// Real slugs this repo produced from Vietnamese objectives whose
	// diacritics Slugify treated as word breaks.
	for _, slug := range []string{"l-m-th-n", "m-r-ng-repo"} {
		if !docmeta.LooksTransliterated(slug) {
			t.Errorf("LooksTransliterated(%q) = false, want true", slug)
		}
	}
}

func TestLooksTransliteratedAcceptsRealEnglishSlugs(t *testing.T) {
	for _, slug := range []string{
		"restructure-vibe-agent-docs",
		"add-retry-ceiling-webhook",
		"fix-web-graph-tab",
		"by-the-way",   // short real English words without a,e,i,o,u
		"mcp-token-t4", // technical abbreviation + version marker, not a language issue
		"mcp-token-t5",
	} {
		if docmeta.LooksTransliterated(slug) {
			t.Errorf("LooksTransliterated(%q) = true, want false", slug)
		}
	}
}
