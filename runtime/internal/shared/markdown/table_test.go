package markdown

import "testing"

func TestParseFirstTable(t *testing.T) {
	text := "| a | b |\n|---|---|\n| one | two |\n| three | four |\n\nprose"
	rows := ParseFirstTable(text)
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[0].Line != 3 || rows[0].Cells[0] != "one" {
		t.Fatalf("first row = %+v", rows[0])
	}
}

func TestLinkTargetAndAssetSlug(t *testing.T) {
	got := LinkTarget("[skill](../skills/foo/SKILL.md)")
	if got != "../skills/foo/SKILL.md" {
		t.Fatalf("LinkTarget = %q", got)
	}
	if AssetSlug(got) != "foo" {
		t.Fatalf("AssetSlug = %q", AssetSlug(got))
	}
}

func TestLinkTargets(t *testing.T) {
	got := LinkTargets("[a](one.md) and [b](two.md)")
	if len(got) != 2 || got[0] != "one.md" || got[1] != "two.md" {
		t.Fatalf("LinkTargets = %v", got)
	}
}
