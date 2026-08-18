package view

import (
	"strings"
	"testing"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/session"
)

func TestBuildEventDetailSummaryListsCoreFields(t *testing.T) {
	row := EventRow{
		Seq:         3,
		Role:        "tool",
		Kind:        session.FilterTool,
		Source:      session.SourceHook,
		Type:        session.TypeToolUse,
		Client:      "cursor",
		Tool:        "Read",
		Command:     "docs/SPEC.md",
		EventName:   "ToolUse",
		At:          time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC),
		Summary:     "Read",
		Body:        "docs/SPEC.md",
		PayloadJSON: `{"model":"local-test","durationMs":12,"client":"cursor","tool":"Read"}`,
		HasUsage:    true,
		Usage:       &session.Usage{Input: 1, Output: 2},
		Failed:      false,
		HostGap:     true,
		Redacted:    false,
	}
	detail := buildEventDetail(row)
	for _, want := range []string{
		"Time", "Type", "Role", "Kind", "Source", "Seq", "Client", "Tool",
		"Command", "Event", "Status", "Failed", "Host gap", "Redacted",
		"Model", "Tokens", "Duration", "Body",
		"cursor", "Read", "docs/SPEC.md", "local-test", "12 ms",
		"data-testid=\"model-name\"", "data-testid=\"inspector-tokens\"",
	} {
		if !strings.Contains(detail.Summary, want) {
			t.Fatalf("summary missing %q:\n%s", want, detail.Summary)
		}
	}
	if !strings.Contains(detail.Payload, `data-testid="inspector-payload"`) {
		t.Fatalf("payload pane missing inspector-payload testid:\n%s", detail.Payload)
	}
}
