package domain

import "testing"

func TestLooksLikeFileRef(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"docs/harness-improvement/SPEC.md", true},
		{"loop-and-graph-engineering.md", true},
		{".ai-agents/references/loop-and-graph-engineering.md", true},
		{"runtime/internal/web/app/files.go", true},
		{"Dockerfile", true},
		{"go.mod", true},
		{"vibe-checks.yaml", true},
		{"go test ./...", false},
		{"fmt.Println", false},
		{"https://example.com/x.md", false},
		{"", false},
		{"README", false},
	}
	for _, tc := range tests {
		got := LooksLikeFileRef(tc.in)
		if got != tc.want {
			t.Errorf("LooksLikeFileRef(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
