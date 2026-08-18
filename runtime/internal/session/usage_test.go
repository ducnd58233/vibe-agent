package session

import "testing"

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
