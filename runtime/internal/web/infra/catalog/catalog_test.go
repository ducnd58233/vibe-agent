package catalog

import (
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
