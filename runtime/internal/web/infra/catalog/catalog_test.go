package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

func testToolkitRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestLoadBuildCommand(t *testing.T) {
	idx, err := Load(testToolkitRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	var build *Entry
	for i := range idx.Commands {
		if idx.Commands[i].Slug == "build" {
			build = &idx.Commands[i]
			break
		}
	}
	if build == nil {
		t.Fatal("build command missing from catalog")
	}
	if build.Insert != "/build" {
		t.Fatalf("insert = %q", build.Insert)
	}
	if build.Description == "" {
		t.Fatal("expected description")
	}
}

func TestSearchCommandsBuild(t *testing.T) {
	idx, err := Load(testToolkitRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	hits := idx.SearchCommands("build")
	if len(hits) == 0 {
		t.Fatal("expected build match")
	}
	found := false
	for _, hit := range hits {
		if hit.Slug == "build" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("hits = %+v", hits)
	}
}

func TestSearchCommandsEmptyReturnsAll(t *testing.T) {
	idx, err := Load(testToolkitRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.SearchCommands("")) == 0 {
		t.Fatal("empty query should list commands")
	}
}

func TestSearchUnknownReturnsEmpty(t *testing.T) {
	idx, err := Load(testToolkitRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.SearchCommands("zzzz-not-a-command-xyzzy")) != 0 {
		t.Fatal("expected empty for unknown query")
	}
	if len(idx.SearchSkills("zzzz-not-a-skill-xyzzy")) != 0 {
		t.Fatal("expected empty for unknown query")
	}
}

func TestLoadForWorkspaceMergesConsumerCommands(t *testing.T) {
	ws := t.TempDir()
	dir := filepath.Join(ws, ".cursor", "commands")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: vibe-unique-test-cmd\ndescription: Unique catalog fixture\n---\n\n# test\n"
	if err := os.WriteFile(filepath.Join(dir, "vibe-unique-test-cmd.md"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	idx, err := LoadForWorkspace(ws, testToolkitRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	var found *Entry
	for i := range idx.Commands {
		if idx.Commands[i].Slug == "vibe-unique-test-cmd" {
			found = &idx.Commands[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected consumer vibe-unique-test-cmd in catalog")
	}
	if found.Insert != "/vibe-unique-test-cmd" {
		t.Fatalf("insert = %q", found.Insert)
	}
	if idx.SearchCommands("build") == nil {
		t.Fatal("toolkit commands must still be present")
	}
}
