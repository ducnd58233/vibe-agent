package runpath_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/runpath"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

func TestAllocateThenResolveRoundTrip(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)

	entry, err := runpath.Allocate(root, "demo-slug", now)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Date != "2026-08-21" || entry.Version != 1 || entry.Slug != "demo-slug" {
		t.Fatalf("entry = %+v", entry)
	}

	got, err := runpath.Resolve(root, "demo-slug")
	if err != nil {
		t.Fatal(err)
	}
	if got != entry {
		t.Fatalf("Resolve = %+v, want %+v", got, entry)
	}

	docs, err := runpath.DocsDir(root, "demo-slug")
	if err != nil {
		t.Fatal(err)
	}
	wantDocs := workspace.DocsDirAt(root, "2026-08-21", "demo-slug", 1)
	if docs != wantDocs {
		t.Fatalf("DocsDir = %q, want %q", docs, wantDocs)
	}

	runDir, err := runpath.RunDir(root, "demo-slug")
	if err != nil {
		t.Fatal(err)
	}
	wantRun := workspace.RunDirAt(root, "2026-08-21", "demo-slug", 1)
	if runDir != wantRun {
		t.Fatalf("RunDir = %q, want %q", runDir, wantRun)
	}
}

func TestAllocateBumpsGlobalVersionAcrossDates(t *testing.T) {
	root := t.TempDir()
	day1 := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)

	first, err := runpath.Allocate(root, "demo-slug", day1)
	if err != nil {
		t.Fatal(err)
	}
	second, err := runpath.Allocate(root, "demo-slug", day2)
	if err != nil {
		t.Fatal(err)
	}
	if first.Version != 1 || second.Version != 2 {
		t.Fatalf("versions = %d then %d, want 1 then 2", first.Version, second.Version)
	}
	if second.Date != "2026-08-22" {
		t.Fatalf("second date = %q, want 2026-08-22", second.Date)
	}
}

func TestResolveScansDiskWhenIndexMissing(t *testing.T) {
	root := t.TempDir()
	path := workspace.RunDirAt(root, "2026-08-20", "scanned", 3)
	if err := os.MkdirAll(path, 0o750); err != nil {
		t.Fatal(err)
	}
	// Lower version on another date must not win.
	if err := os.MkdirAll(workspace.DocsDirAt(root, "2026-08-21", "scanned", 2), 0o750); err != nil {
		t.Fatal(err)
	}

	entry, err := runpath.Resolve(root, "scanned")
	if err != nil {
		t.Fatal(err)
	}
	if entry.Version != 3 || entry.Date != "2026-08-20" {
		t.Fatalf("entry = %+v, want version 3 on 2026-08-20", entry)
	}
}

func TestResolveIgnoresFlatLegacyDirs(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "tmp", "legacy-slug"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs", "legacy-slug"), 0o750); err != nil {
		t.Fatal(err)
	}

	_, err := runpath.Resolve(root, "legacy-slug")
	if !errors.Is(err, runpath.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound (no flat dual-read)", err)
	}
}

func TestBeginRefusesIndexedRuns(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	if _, err := runpath.Allocate(root, "taken", now); err != nil {
		t.Fatal(err)
	}
	if _, err := runpath.Begin(root, "taken", now); err == nil {
		t.Fatal("Begin accepted an indexed slug")
	}
}

func TestBeginAllocatesFirstRevision(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	entry, err := runpath.Begin(root, "fresh", now)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Version != 1 || entry.Date != "2026-08-21" {
		t.Fatalf("entry = %+v", entry)
	}
}

func TestBeginRefusesCaseOnlyCollision(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	if _, err := runpath.Allocate(root, "MyFeature", now); err != nil {
		t.Fatal(err)
	}
	if _, err := runpath.Begin(root, "myfeature", now); err == nil {
		t.Fatal("Begin accepted a slug differing only in case from an existing one")
	}
}

func TestBeginAllowsUnrelatedSlugsDifferingInCaseFromNothing(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	if _, err := runpath.Allocate(root, "AlphaFeature", now); err != nil {
		t.Fatal(err)
	}
	if _, err := runpath.Begin(root, "BetaFeature", now); err != nil {
		t.Fatalf("Begin refused an unrelated slug: %v", err)
	}
}

func TestExistingSlugsFindsIndexedAndScannedEntries(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	if _, err := runpath.Allocate(root, "Indexed", now); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(workspace.RunDirAt(root, "2026-08-20", "ScannedOnly", 1), 0o750); err != nil {
		t.Fatal(err)
	}

	got, err := runpath.ExistingSlugs(root)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"Indexed": true, "ScannedOnly": true}
	for _, slug := range got {
		delete(want, slug)
	}
	if len(want) != 0 {
		t.Fatalf("ExistingSlugs = %v, missing %v", got, want)
	}
}

func TestCheckRevisionRejectsBadSegments(t *testing.T) {
	if workspace.DocsDirAt("/w", "not-a-date", "slug", 1) != "" {
		t.Fatal("bad date should yield empty DocsDirAt")
	}
	if workspace.RunDirAt("/w", "2026-08-21", "slug", 0) != "" {
		t.Fatal("version 0 should yield empty RunDirAt")
	}
}
