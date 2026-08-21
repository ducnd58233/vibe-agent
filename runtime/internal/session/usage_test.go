package session

import (
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/testutil"
)

func TestParseUsageReadsSplitAndTotal(t *testing.T) {
	split := ParseUsage(map[string]any{
		"input_tokens":            float64(12),
		"output_tokens":           float64(4),
		"cache_read_input_tokens": float64(3),
	})
	if split == nil || split.Input != 12 || split.Output != 4 || split.CacheRead != 3 {
		t.Fatalf("split = %+v", split)
	}
	nested := ParseUsage(map[string]any{
		"usage": map[string]any{"total_tokens": float64(88)},
	})
	if nested == nil || nested.Total != 88 || nested.Input != 0 {
		t.Fatalf("nested total = %+v", nested)
	}
	if ParseUsage(map[string]any{"other": "x"}) != nil {
		t.Fatal("empty usage should be nil")
	}
	camel := ParseUsage(map[string]any{
		"inputTokens":     float64(57151),
		"outputTokens":    float64(33),
		"cacheReadTokens": float64(384),
	})
	if camel == nil || camel.Input != 57151 || camel.Output != 33 || camel.CacheRead != 384 {
		t.Fatalf("camel = %+v", camel)
	}
}

// The runtime owns no model call, so a session log is the only place a token
// figure exists. Summing it is what makes a token budget enforceable at all.
func TestTokensUsedSumsWhatTheHostsReported(t *testing.T) {
	root := t.TempDir()
	testutil.EnsureRunIndex(t, root, "demo")
	path := LogPath(root, "demo")
	records := []Record{
		{Type: TypeTranscriptMessage, Source: SourcePrint, Role: "assistant", Body: "one",
			Usage: &Usage{Input: 100, Output: 50}},
		{Type: TypeTranscriptMessage, Source: SourcePrint, Role: "assistant", Body: "two",
			Usage: &Usage{Total: 30}},
		// No usage at all: hosts disagree about which turns carry counts, and a
		// log full of untotalled turns is normal rather than broken.
		{Type: TypePromptSubmit, Source: SourceHook, Body: "three"},
	}
	for _, record := range records {
		if _, err := Append(path, record); err != nil {
			t.Fatal(err)
		}
	}

	total, err := TokensUsed(path)
	if err != nil {
		t.Fatal(err)
	}
	if total != 180 {
		t.Errorf("total = %d, want 180 (100+50 then 30)", total)
	}
}

// A cache read is the cost being avoided rather than paid, so it stays out.
func TestTokensUsedLeavesCacheReadsOut(t *testing.T) {
	root := t.TempDir()
	testutil.EnsureRunIndex(t, root, "demo")
	path := LogPath(root, "demo")
	if _, err := Append(path, Record{
		Type: TypeTranscriptMessage, Source: SourcePrint, Role: "assistant", Body: "cached",
		Usage: &Usage{Input: 10, Output: 5, CacheRead: 9000},
	}); err != nil {
		t.Fatal(err)
	}
	total, err := TokensUsed(path)
	if err != nil {
		t.Fatal(err)
	}
	if total != 15 {
		t.Errorf("total = %d, want 15", total)
	}
}

func TestTokensUsedOnAnAbsentLogIsZero(t *testing.T) {
	root := t.TempDir()
	testutil.EnsureRunIndex(t, root, "nothing-here")
	total, err := TokensUsed(LogPath(root, "nothing-here"))
	if err != nil {
		t.Fatalf("an absent log errored: %v", err)
	}
	if total != 0 {
		t.Errorf("total = %d, want 0", total)
	}
}
