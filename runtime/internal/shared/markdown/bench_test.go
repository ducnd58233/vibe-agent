package markdown

import (
	"strings"
	"testing"
)

func benchRouterTable() string {
	var b strings.Builder
	b.WriteString("| Intent / use case | Skill folder | When to invoke |\n")
	b.WriteString("|-------------------|--------------|----------------|\n")
	for i := 0; i < 40; i++ {
		b.WriteString("| workflow item ")
		b.WriteString(strings.Repeat("x", i%5))
		b.WriteString(" | [`skill-")
		b.WriteString(strings.Repeat("a", 8))
		b.WriteString("`](skills/skill-")
		b.WriteString(strings.Repeat("a", 8))
		b.WriteString("/SKILL.md) | when needed |\n")
	}
	b.WriteString("\nTrailing prose after the table.\n")
	return b.String()
}

func BenchmarkParseFirstTable(b *testing.B) {
	text := benchRouterTable()
	if rows := ParseFirstTable(text); len(rows) != 40 {
		b.Fatalf("setup rows = %d, want 40", len(rows))
	}
	b.ReportAllocs()
	for b.Loop() {
		_ = ParseFirstTable(text)
	}
}
