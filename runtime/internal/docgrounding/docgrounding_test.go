package docgrounding_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/docgrounding"
)

func write(t *testing.T, root, name, body string) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCheckFlagsADanglingPath(t *testing.T) {
	root := t.TempDir()
	write(t, root, "runtime/cmd/real.go", "package main\n")
	doc := write(t, root, "docs/demo.md", "See `runtime/cmd/real.go` and `runtime/cmd/nonexistent.go` for details.\n")

	issues, err := docgrounding.Check(root, doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 1 {
		t.Fatalf("issues = %+v, want exactly 1", issues)
	}
	if issues[0].Path != "runtime/cmd/nonexistent.go" {
		t.Fatalf("issue = %+v", issues[0])
	}
}

func TestCheckAcceptsRealPaths(t *testing.T) {
	root := t.TempDir()
	write(t, root, "runtime/cmd/real.go", "package main\n")
	doc := write(t, root, "docs/demo.md", "See `runtime/cmd/real.go` for the implementation.\n")

	issues, err := docgrounding.Check(root, doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Fatalf("issues = %+v, want none", issues)
	}
}

func TestCheckSkipsPlaceholderPatterns(t *testing.T) {
	root := t.TempDir()
	doc := write(t, root, "docs/demo.md", "Write to `docs/<date>/<slug>/<version>/SPEC-<date>.md` per convention.\n")

	issues, err := docgrounding.Check(root, doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Fatalf("a templated placeholder path should not be flagged: %+v", issues)
	}
}

func TestCheckSkipsExplicitlyMarkedFuturePaths(t *testing.T) {
	root := t.TempDir()
	doc := write(t, root, "docs/demo.md", "Planned: `runtime/cmd/not-built-yet.go` will add this.\n")

	issues, err := docgrounding.Check(root, doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Fatalf("a path explicitly marked planned should not be flagged: %+v", issues)
	}
}

func TestCheckSkipsNonPathBackticks(t *testing.T) {
	root := t.TempDir()
	doc := write(t, root, "docs/demo.md", "Pass `--slug` before the objective, not `currentNode`, and never `human_event`.\n")

	issues, err := docgrounding.Check(root, doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Fatalf("flag names and bare identifiers should not be flagged: %+v", issues)
	}
}

func TestCheckSkipsSlashCommandReferences(t *testing.T) {
	root := t.TempDir()
	doc := write(t, root, "docs/demo.md", "Run `/vibe-auto` or `/review` before shipping.\n")

	issues, err := docgrounding.Check(root, doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Fatalf("a slash-command reference should not be flagged as a path: %+v", issues)
	}
}

func TestCheckSkipsGlobPatterns(t *testing.T) {
	root := t.TempDir()
	doc := write(t, root, "docs/demo.md", "See `.ai-agents/commands/*.md` and `docs/2026-08-21/some-slug/1/*`.\n")

	issues, err := docgrounding.Check(root, doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Fatalf("a glob pattern is not a literal path and should not be flagged: %+v", issues)
	}
}

func TestCheckSkipsBraceExpansionPatterns(t *testing.T) {
	root := t.TempDir()
	doc := write(t, root, "docs/demo.md", "See `.ai-agents/commands/{spec,plan,build}.md` for the set.\n")

	issues, err := docgrounding.Check(root, doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Fatalf("a brace-expansion pattern is not a literal path and should not be flagged: %+v", issues)
	}
}

func TestCheckSkipsEllipsisPlaceholders(t *testing.T) {
	root := t.TempDir()
	doc := write(t, root, "docs/demo.md", "Evidence lives under `.agent-state/runs/.../manifest.json`.\n")

	issues, err := docgrounding.Check(root, doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Fatalf("an ellipsis placeholder is not a literal path and should not be flagged: %+v", issues)
	}
}

func TestCheckRefusesToEscapeTheWorkspaceRoot(t *testing.T) {
	root := t.TempDir()
	// A real file that exists just outside root, so a naive Stat after
	// filepath.Join would find it and wrongly call the reference grounded.
	outside := filepath.Join(filepath.Dir(root), "outside-marker.txt")
	if err := os.WriteFile(outside, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Remove(outside)
	}()

	doc := write(t, root, "docs/demo.md", "See `../"+filepath.Base(outside)+"` for details.\n")

	issues, err := docgrounding.Check(root, doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 1 {
		t.Fatalf("a path escaping the workspace root must be flagged, not resolved outside it: issues = %+v", issues)
	}
}

func TestCheckSkipsFencedCodeBlocks(t *testing.T) {
	root := t.TempDir()
	doc := write(t, root, "docs/demo.md", "Example:\n\n```text\nruntime/cmd/does-not-exist-and-thats-fine.go\n```\n")

	issues, err := docgrounding.Check(root, doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Fatalf("a fenced code example should not be flagged: %+v", issues)
	}
}

func TestCheckSkipsGitBranchNames(t *testing.T) {
	root := t.TempDir()
	doc := write(t, root, "docs/demo.md", "**Branch:** `feat/some-slug-t1-thing`\n")

	issues, err := docgrounding.Check(root, doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Fatalf("a conventional-commit-shaped branch name is not a path and should not be flagged: %+v", issues)
	}
}

func TestCheckStillFlagsADanglingDocsDirectory(t *testing.T) {
	root := t.TempDir()
	write(t, root, "docs/2026-09-24/real-slug/1/SPEC-2026-09-24.md", "# real\n")
	doc := write(t, root, "docs/demo.md", "See `docs/2026-09-24/real-slug/1` and `docs/2026-09-24/fake-slug/1` for details.\n")

	issues, err := docgrounding.Check(root, doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 1 || issues[0].Path != "docs/2026-09-24/fake-slug/1" {
		t.Fatalf("a bare docs/<date>/<slug>/<version> directory reference must still be checked - \"docs\" is a real top-level directory, not a branch prefix: issues = %+v", issues)
	}
}
