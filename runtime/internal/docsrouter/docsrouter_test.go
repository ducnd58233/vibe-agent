package docsrouter_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/docsrouter"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/runpath"
)

func writeSpec(t *testing.T, root, date, slug string, version int, title string) {
	t.Helper()
	dir := filepath.Join(root, "docs", date, slug, strconv.Itoa(version))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	body := "---\nslug: " + slug + "\ndate: " + date + "\nversion: " + strconv.Itoa(version) + "\n---\n\n# Spec: " + title + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SPEC-"+date+".md"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	err := runpath.SaveIndex(root, runpath.Entry{Slug: slug, Date: date, Version: version})
	if err != nil {
		t.Fatal(err)
	}
}

func TestGenerateListsSlugWithSpecTitle(t *testing.T) {
	root := t.TempDir()
	writeSpec(t, root, "2026-08-21", "docs-tmp-versioned-paths", 1, "Versioned docs/tmp layout")

	out, err := docsrouter.Generate(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "docs-tmp-versioned-paths") {
		t.Fatalf("router missing slug: %s", out)
	}
	if !strings.Contains(out, "Versioned docs/tmp layout") {
		t.Fatalf("router missing title: %s", out)
	}
	if !strings.Contains(out, "docs/2026-08-21/docs-tmp-versioned-paths/1/") {
		t.Fatalf("router missing path: %s", out)
	}
}

func TestGenerateFallsBackToManifestGoalWithNoSpec(t *testing.T) {
	root := t.TempDir()
	// A slug with a run-index entry but no SPEC file (e.g. a no-docs-* run).
	err := runpath.SaveIndex(root, runpath.Entry{Slug: "no-docs-fix-typo", Date: "2026-09-24", Version: 1})
	if err != nil {
		t.Fatal(err)
	}
	out, err := docsrouter.Generate(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "no-docs-fix-typo") {
		t.Fatalf("router missing no-docs slug: %s", out)
	}
	if !strings.Contains(out, "no SPEC written") {
		t.Fatalf("router should note the missing SPEC: %s", out)
	}
}

func TestTitleForMatchesGenerate(t *testing.T) {
	root := t.TempDir()
	writeSpec(t, root, "2026-08-21", "docs-tmp-versioned-paths", 1, "Versioned docs/tmp layout")

	got := docsrouter.TitleFor(root, "docs-tmp-versioned-paths")
	if got != "Versioned docs/tmp layout" {
		t.Fatalf("TitleFor = %q", got)
	}
}

func TestTitleForUnknownSlugFallsBackCleanly(t *testing.T) {
	root := t.TempDir()
	got := docsrouter.TitleFor(root, "never-seen-this-slug")
	if got != "(no SPEC written for this slug)" {
		t.Fatalf("TitleFor for an unknown slug = %q", got)
	}
}

func TestGenerateIsDeterministicAcrossRuns(t *testing.T) {
	root := t.TempDir()
	writeSpec(t, root, "2026-08-21", "zeta-slug", 1, "Zeta work")
	writeSpec(t, root, "2026-08-20", "alpha-slug", 1, "Alpha work")

	first, err := docsrouter.Generate(root)
	if err != nil {
		t.Fatal(err)
	}
	second, err := docsrouter.Generate(root)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("generation is not deterministic:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
	// Alphabetical by slug, so alpha-slug's row comes before zeta-slug's.
	if strings.Index(first, "alpha-slug") > strings.Index(first, "zeta-slug") {
		t.Fatalf("expected alpha-slug before zeta-slug:\n%s", first)
	}
}
