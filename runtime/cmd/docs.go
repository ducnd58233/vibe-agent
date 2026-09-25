package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ducnd58233/vibe-agent/runtime/internal/citations"
	"github.com/ducnd58233/vibe-agent/runtime/internal/docgrounding"
	"github.com/ducnd58233/vibe-agent/runtime/internal/docsrouter"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

// docsCommand dispatches docs/ subcommands.
func docsCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("docs needs a subcommand: router, check-claims, check-citations")
	}
	switch args[0] {
	case "router":
		return docsRouter(args[1:])
	case "check-claims":
		return docsCheckClaims(args[1:])
	case "check-citations":
		return docsCheckCitations(args[1:])
	default:
		return fmt.Errorf("unknown docs subcommand %q; try router, check-claims, check-citations", args[0])
	}
}

// docsRouter regenerates docs/ROUTER.md from this workspace's run-index and
// docs/ tree. Safe to run any time; a no-op regeneration produces an
// identical file, so it is not a deliverable a person needs to review as a
// diff each time.
func docsRouter(args []string) error {
	flags := newFlagSet("docs router")
	paths := addRootFlags(flags)
	if err := flags.Parse(args); err != nil {
		return err
	}
	workspaceRoot, _, err := paths.resolve()
	if err != nil {
		return err
	}
	content, err := docsrouter.Generate(workspaceRoot)
	if err != nil {
		return err
	}
	path := filepath.Join(workspaceRoot, workspace.DocsDirName, "ROUTER.md")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	fmt.Printf("wrote %s\n", path)
	return nil
}

// docsCheckClaims is a /review warning, not a vibe-checks.yaml gate: it flags
// a backtick-quoted path in a doc that does not resolve in the repo tree, so
// a reviewer spends attention on what a machine cannot say instead of
// re-deriving what it can. Exits non-zero on a finding so a caller can choose
// to treat it as a warning or a blocker.
func docsCheckClaims(args []string) error {
	flags := newFlagSet("docs check-claims")
	paths := addRootFlags(flags)
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return fmt.Errorf("docs check-claims needs exactly one markdown file path")
	}
	workspaceRoot, _, err := paths.resolve()
	if err != nil {
		return err
	}
	target := flags.Arg(0)
	issues, err := docgrounding.Check(workspaceRoot, target)
	if err != nil {
		return err
	}
	if len(issues) == 0 {
		fmt.Printf("ok    %s: every backtick-quoted path resolves\n", target)
		return nil
	}
	for _, issue := range issues {
		fmt.Printf("WARN  %s:%d: %q does not resolve in the repo tree\n", target, issue.Line, issue.Path)
	}
	return fmt.Errorf("%d unresolved path reference(s) in %s", len(issues), target)
}

// docsCheckCitations requests every http(s) URL a markdown file cites, outside
// fenced code, and fails on any that does not resolve. The same check gates the
// research nodes at checkpoint time.
func docsCheckCitations(args []string) error {
	flags := newFlagSet("docs check-citations")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return fmt.Errorf("docs check-citations needs exactly one markdown file path")
	}
	target := flags.Arg(0)
	raw, err := os.ReadFile(filepath.Clean(target))
	if err != nil {
		return err
	}
	urls := citations.Extract(raw)
	failures := citations.Default().Check(context.Background(), urls)
	for _, failure := range failures {
		fmt.Printf("FAIL  %s\n", failure)
	}
	if len(failures) > 0 {
		return fmt.Errorf("%d of %d cited URL(s) in %s do not resolve", len(failures), len(urls), target)
	}
	fmt.Printf("ok    %s: all %d cited URL(s) resolve\n", target, len(urls))
	return nil
}
