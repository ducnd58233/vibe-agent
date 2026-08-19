package graph

import (
	"os"
	"testing"
)

func BenchmarkLoadGoalDelivery(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		g, err := Load(repoGraph)
		if err != nil {
			b.Fatal(err)
		}
		if g.Metadata.ID != "goal-delivery" {
			b.Fatalf("id = %q", g.Metadata.ID)
		}
	}
}

func BenchmarkParseGoalDeliveryYAML(b *testing.B) {
	raw, err := os.ReadFile(repoGraph)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		g, err := Parse(raw)
		if err != nil {
			b.Fatal(err)
		}
		if g.Metadata.ID != "goal-delivery" {
			b.Fatalf("id = %q", g.Metadata.ID)
		}
	}
}
