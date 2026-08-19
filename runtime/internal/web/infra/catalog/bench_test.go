package catalog

import (
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/testutil"
)

func BenchmarkLoadToolkitCatalog(b *testing.B) {
	root := testutil.ToolkitRoot(b)
	b.ReportAllocs()
	for b.Loop() {
		idx, err := Load(root)
		if err != nil {
			b.Fatal(err)
		}
		if len(idx.Skills) == 0 || len(idx.Commands) == 0 {
			b.Fatal("expected non-empty catalog")
		}
	}
}
