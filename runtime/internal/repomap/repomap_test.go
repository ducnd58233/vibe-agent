package repomap

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildRanksReferencedFileAboveUnrelated(t *testing.T) {
	root := filepath.Join("testdata", "fixture")
	result, err := Build(t.Context(), root, Options{Budget: 2000})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if result.Text == "" {
		t.Fatal("empty map text")
	}
	aIdx := strings.Index(result.Text, "pkg_a/a.go")
	cIdx := strings.Index(result.Text, "pkg_c/c.go")
	if aIdx < 0 {
		t.Fatalf("expected pkg_a in map:\n%s", result.Text)
	}
	if cIdx >= 0 && aIdx > cIdx {
		t.Fatalf("pkg_a should rank above pkg_c:\n%s", result.Text)
	}
	if !strings.Contains(result.Text, "Helper") {
		t.Fatalf("expected Helper definition in map:\n%s", result.Text)
	}
}

func TestBuildRespectsBudget(t *testing.T) {
	root := filepath.Join("testdata", "fixture")
	result, err := Build(t.Context(), root, Options{Budget: 15})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if result.Tokens > 15 {
		t.Fatalf("tokens=%d over budget 15; text:\n%s", result.Tokens, result.Text)
	}
	if result.FilesIncluded+result.FilesOmitted == 0 {
		t.Fatal("expected included/omitted counts")
	}
}

func TestFocusBiasesMatchingPaths(t *testing.T) {
	root := filepath.Join("testdata", "fixture")
	focused, err := Build(t.Context(), root, Options{Budget: 2000, Focus: "pkg_c"})
	if err != nil {
		t.Fatalf("Build focus: %v", err)
	}
	cIdx := strings.Index(focused.Text, "pkg_c/c.go")
	aIdx := strings.Index(focused.Text, "pkg_a/a.go")
	if cIdx < 0 {
		t.Fatalf("focus map missing pkg_c:\n%s", focused.Text)
	}
	if aIdx < 0 {
		t.Fatalf("focus must not exclude pkg_a:\n%s", focused.Text)
	}
	if cIdx > aIdx {
		t.Fatalf("focus should put pkg_c above pkg_a:\n%s", focused.Text)
	}
}
