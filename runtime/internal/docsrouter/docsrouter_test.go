package docsrouter_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/ducnd58233/vibe-agent/runtime/internal/docsrouter"
	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/runpath"
)

func writeSpec(t *testing.T, root, date, slug string, version int, title string) {
	t.Helper()
	dir := filepath.Join(root, "docs", date, slug, strconv.Itoa(version))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".agent-state", "runs", date, slug, strconv.Itoa(version)), 0o750); err != nil {
		t.Fatal(err)
	}
	body := "---\nslug: " + slug + "\ndate: " + date + "\nversion: " + strconv.Itoa(version) + "\n---\n\n# Spec: " + title + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SPEC-"+date+".md"), []byte(body), 0o600); err != nil {
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
	// A slug with a run directory but no SPEC file (e.g. a no-docs-* run).
	if err := os.MkdirAll(filepath.Join(root, ".agent-state", "runs", "2026-09-24", "no-docs-fix-typo", "1"), 0o750); err != nil {
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

func TestFallbackTitleTruncatesOnRuneBoundaries(t *testing.T) {
	// A goal long enough to truncate, with a multi-byte rune sitting exactly
	// where a byte-index slice at 80 would split it. Real data: this repo's
	// own l-m-th-n and m-r-ng-repo slugs carry Vietnamese goal text.
	goal := strings.Repeat("x", 79) + "ế" + strings.Repeat("y", 20)
	root := t.TempDir()
	if _, err := runpath.Allocate(root, "probe-slug", time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	manifest := &state.Run{
		RunID: "run_probe", GraphID: "goal-delivery", Slug: "probe-slug",
		Goal: goal, SchemaVersion: 1, Status: state.StatusRunning, MaxTransitions: 100,
	}
	if err := state.Save(state.ManifestPath(root, "probe-slug"), manifest); err != nil {
		t.Fatal(err)
	}

	got := docsrouter.TitleFor(root, "probe-slug")
	if !utf8.ValidString(got) {
		t.Fatalf("TitleFor produced invalid UTF-8: %q", got)
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
