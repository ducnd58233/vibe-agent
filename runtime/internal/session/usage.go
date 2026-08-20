package session

import "encoding/json"

// ParseUsage reads host-reported token counts from a JSON object.
//
// Hosts disagree on field names. Cursor and Claude use input_tokens /
// output_tokens; some print streams only report total_tokens. Either shape is
// enough to show a number in the UI. Split in/out is preferred when both exist.
func ParseUsage(raw map[string]any) *Usage {
	if raw == nil {
		return nil
	}
	if nested, ok := raw["usage"].(map[string]any); ok {
		raw = nested
	}
	var usage Usage
	has := false
	if v, ok := usageInt(raw, "input", "input_tokens", "inputTokens"); ok {
		usage.Input = v
		has = true
	}
	if v, ok := usageInt(raw, "output", "output_tokens", "outputTokens"); ok {
		usage.Output = v
		has = true
	}
	if v, ok := usageInt(raw, "cacheRead", "cache_read", "cache_read_input_tokens", "cacheReadTokens"); ok {
		usage.CacheRead = v
		has = true
	}
	if v, ok := usageInt(raw, "total", "total_tokens"); ok {
		usage.Total = v
		has = true
	}
	if !has {
		return nil
	}
	return &usage
}

// Reported is true when at least one token field is a positive count.
func (u *Usage) Reported() bool {
	if u == nil {
		return false
	}
	return u.Input > 0 || u.Output > 0 || u.CacheRead > 0 || u.Total > 0
}

func usageInt(raw map[string]any, keys ...string) (int, bool) {
	for _, key := range keys {
		value, ok := raw[key]
		if !ok {
			continue
		}
		switch n := value.(type) {
		case float64:
			return int(n), true
		case int:
			return n, true
		case int64:
			return int(n), true
		case json.Number:
			v, err := n.Int64()
			if err != nil {
				return 0, false
			}
			return int(v), true
		}
	}
	return 0, false
}

// TokensUsed sums the host-reported token counts in a session log.
//
// The runtime owns no model call, so this is the only place a token figure
// exists: whatever the hosts reported, as they reported it. Input plus output
// where both are present, total where only that is, and cache reads left out
// because a cache hit is the cost being avoided rather than paid.
//
// A record with no usage contributes nothing rather than being an error. Hosts
// disagree about which turns carry counts, and a log full of untotalled turns
// is normal rather than broken.
func TokensUsed(logPath string) (int, error) {
	events, err := Replay(logPath)
	if err != nil {
		return 0, err
	}
	total := 0
	for _, event := range events {
		if len(event.Payload) == 0 {
			continue
		}
		var payload Payload
		if json.Unmarshal(event.Payload, &payload) != nil {
			continue
		}
		if payload.Usage == nil {
			continue
		}
		if payload.Usage.Input > 0 || payload.Usage.Output > 0 {
			total += payload.Usage.Input + payload.Usage.Output
			continue
		}
		total += payload.Usage.Total
	}
	return total, nil
}
