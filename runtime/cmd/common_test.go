package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// setHome points os.UserHomeDir at dir. It reads different variables per
// platform, so setting only HOME would make these tests pass on Linux and do
// nothing on Windows, which is the platform the global install was built for.
func setHome(t *testing.T, dir string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", dir)
		return
	}
	t.Setenv("HOME", dir)
}

// isolate cuts the test off from the developer's own machine.
//
// Without this, a subtest asserting the workspace fallback passes on a clean
// checkout and fails on any machine where scripts/install-global has run,
// because discoverToolkit finds the real ~/.vibe-agent. Every subtest calls it,
// including the ones that then set their own values.
func isolate(t *testing.T) {
	t.Helper()
	t.Setenv(ToolkitEnv, "")
	setHome(t, t.TempDir())
}

// withAssets builds a directory that looks like a toolkit root.
func withAssets(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".ai-agents", "graphs"), 0o750); err != nil {
		t.Fatalf("create assets in %s: %v", dir, err)
	}
	return dir
}

// chdir moves into dir for one test. Discovery reads the process's working
// directory, which is the thing under test, so it has to actually change.
func chdir(t *testing.T, dir string) {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir(%s): %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
}

// resolved returns what a command with no --workspace would use.
func resolved(t *testing.T) string {
	t.Helper()
	root, _, err := discoverWorkspace()
	if err != nil {
		t.Fatalf("discoverWorkspace: %v", err)
	}
	actual, err := filepath.EvalSymlinks(root)
	if err != nil {
		return root
	}
	return actual
}

func expectRoot(t *testing.T, want, got string) {
	t.Helper()
	resolvedWant, err := filepath.EvalSymlinks(want)
	if err != nil {
		resolvedWant = want
	}
	if got != resolvedWant {
		t.Errorf("discovered %s, want %s", got, resolvedWant)
	}
}

// Discovery exists because three hosts out of four publish no project-directory
// variable and do not document the working directory a hook runs in. Trusting
// cwd made the workspace a guess there, and a wrong guess is silent: the hook
// runs, exits 0, and finds no runs and no memories in a directory that has
// none.
func TestDiscoverWorkspaceWalksUpToTheCheckout(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o750); err != nil {
		t.Fatalf("create .git: %v", err)
	}
	deep := filepath.Join(root, "packages", "web", "src")
	if err := os.MkdirAll(deep, 0o750); err != nil {
		t.Fatalf("create subdirectory: %v", err)
	}

	chdir(t, deep)
	expectRoot(t, root, resolved(t))
}

// The trap this test exists for: AGENTS.md, CLAUDE.md and CURSOR.md are all
// allowed to nest, one per subdirectory. A single-pass search would stop at the
// nearest one and call a package directory the workspace, putting run state and
// memory two levels below where they belong.
func TestANestedRulesFileDoesNotShadowTheCheckout(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o750); err != nil {
		t.Fatalf("create .git: %v", err)
	}
	nested := filepath.Join(root, "packages", "web")
	if err := os.MkdirAll(nested, 0o750); err != nil {
		t.Fatalf("create subdirectory: %v", err)
	}
	for _, name := range []string{"CLAUDE.md", "AGENTS.md", "CURSOR.md"} {
		if err := os.WriteFile(filepath.Join(nested, name), []byte("# local rules\n"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	chdir(t, nested)
	expectRoot(t, root, resolved(t))
}

// The global toolkit install is not a workspace.
//
// scripts/install-global creates ~/.vibe-agent, and treating it as a workspace
// marker made every directory under the home directory that is not inside a
// checkout resolve to the home directory. That is worse than the guess it
// replaced: the old default at least stayed where the hook ran.
func TestTheGlobalToolkitInstallIsNotAWorkspace(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)
	if err := os.MkdirAll(filepath.Join(home, GlobalToolkitDir, ".ai-agents"), 0o750); err != nil {
		t.Fatalf("create global toolkit: %v", err)
	}
	plain := filepath.Join(home, "scratch")
	if err := os.MkdirAll(plain, 0o750); err != nil {
		t.Fatalf("create subdirectory: %v", err)
	}

	chdir(t, plain)
	if got := resolved(t); got == mustEval(t, home) {
		t.Errorf("discovery treated the global toolkit install as a workspace: %s", got)
	}
}

// A workspace that is not a git repository still has to be found. Test fixtures
// usually are not one, and neither is a directory someone unzipped.
//
// This drives nearestAncestorWith rather than discoverWorkspace on purpose. A
// temporary directory always has real ancestors, and one of them holding a .git
// - a dotfiles repository in the home directory, say - would beat the fixture
// legitimately and fail a test that is not about that. The pass being checked
// here is the second one.
func TestARulesFileIsEnoughWithoutAStructuralMarker(t *testing.T) {
	for _, marker := range []string{"AGENTS.md", "CLAUDE.md", "CURSOR.md"} {
		t.Run(marker, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, marker), []byte("# rules\n"), 0o600); err != nil {
				t.Fatalf("write %s: %v", marker, err)
			}
			deep := filepath.Join(root, "a", "b")
			if err := os.MkdirAll(deep, 0o750); err != nil {
				t.Fatalf("create subdirectory: %v", err)
			}

			found, ok := nearestAncestorWith(deep, rulesMarkers)
			if !ok {
				t.Fatalf("%s two levels up was not found from %s", marker, deep)
			}
			expectRoot(t, root, mustEval(t, found))
		})
	}
}

// An explicit --workspace is an instruction, not a hint. Discovery must not
// second-guess it, or a consumer repository mounting the toolkit elsewhere
// would silently get a different root than it asked for.
func TestAnExplicitWorkspaceIsNotDiscoveredOver(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	given := root
	flags := rootFlags{workspace: &given, toolkit: new(string)}

	workspace, _, err := flags.resolve()
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	expectRoot(t, root, mustEval(t, workspace))
}

func mustEval(t *testing.T, path string) string {
	t.Helper()
	actual, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}
	return actual
}

// TestDiscoverToolkitOrder pins the resolution order rather than any single
// case. The order is the contract: local before global, so a repository that
// ships its own assets is never shadowed by a machine-wide install.
func TestDiscoverToolkitOrder(t *testing.T) {
	t.Run("workspace itself when it holds the assets", func(t *testing.T) {
		isolate(t)
		workspace := withAssets(t, t.TempDir())
		if got := discoverToolkit(workspace); got != workspace {
			t.Fatalf("got %q, want the workspace %q", got, workspace)
		}
	})

	t.Run("submodule one level down", func(t *testing.T) {
		isolate(t)
		workspace := t.TempDir()
		sub := withAssets(t, filepath.Join(workspace, ".vibe-agent"))
		if got := discoverToolkit(workspace); got != sub {
			t.Fatalf("got %q, want the submodule %q", got, sub)
		}
	})

	t.Run("git and node_modules are never the toolkit", func(t *testing.T) {
		isolate(t)
		workspace := t.TempDir()
		withAssets(t, filepath.Join(workspace, ".git"))
		withAssets(t, filepath.Join(workspace, "node_modules"))
		if got := discoverToolkit(workspace); got != workspace {
			t.Fatalf("got %q, want the workspace fallback %q", got, workspace)
		}
	})

	t.Run("environment variable when nothing is local", func(t *testing.T) {
		isolate(t)
		workspace := t.TempDir()
		global := withAssets(t, t.TempDir())
		t.Setenv(ToolkitEnv, global)
		if got := discoverToolkit(workspace); got != global {
			t.Fatalf("got %q, want %s %q", got, ToolkitEnv, global)
		}
	})

	t.Run("a local toolkit beats the environment variable", func(t *testing.T) {
		isolate(t)
		workspace := withAssets(t, t.TempDir())
		t.Setenv(ToolkitEnv, withAssets(t, t.TempDir()))
		if got := discoverToolkit(workspace); got != workspace {
			t.Fatalf("got %q, want the local toolkit %q; global must not shadow a vendored one", got, workspace)
		}
	})

	t.Run("an environment variable pointing nowhere is ignored", func(t *testing.T) {
		isolate(t)
		workspace := t.TempDir()
		t.Setenv(ToolkitEnv, filepath.Join(t.TempDir(), "absent"))
		if got := discoverToolkit(workspace); got != workspace {
			t.Fatalf("got %q, want the workspace fallback %q", got, workspace)
		}
	})

	t.Run("the per-user install when nothing else matches", func(t *testing.T) {
		isolate(t)
		home := t.TempDir()
		global := withAssets(t, filepath.Join(home, GlobalToolkitDir))
		setHome(t, home)

		workspace := t.TempDir()
		if got := discoverToolkit(workspace); got != global {
			t.Fatalf("got %q, want the per-user toolkit %q", got, global)
		}
	})

	t.Run("workspace fallback when there is no toolkit anywhere", func(t *testing.T) {
		isolate(t)
		workspace := t.TempDir()
		if got := discoverToolkit(workspace); got != workspace {
			t.Fatalf("got %q, want the workspace %q so the error names a path the caller knows", got, workspace)
		}
	})
}

// TestHoldsAssetsRejectsAFile guards the check that .ai-agents is a directory.
// A file by that name would otherwise pass and every later read would fail with
// a confusing error far from the cause.
func TestNormalizeVolumeUppercasesDriveLetter(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("drive letters are a Windows concept")
	}
	for _, tc := range []struct{ in, want string }{
		{`d:\projects\vibe-agent`, `D:\projects\vibe-agent`},
		{`D:\projects\vibe-agent`, `D:\projects\vibe-agent`},
		{`c:\Users`, `C:\Users`},
	} {
		if got := normalizeVolume(tc.in); got != tc.want {
			t.Errorf("normalizeVolume(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestHoldsAssetsRejectsAFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".ai-agents"), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if holdsAssets(dir) {
		t.Fatal("a file named .ai-agents was accepted as a toolkit root")
	}
}

// The distinction the fallback used to throw away. Both outcomes were a path
// and a nil error, so every caller treated "there is no workspace here" as
// "this is the workspace root" - which is how a fetch came to create an
// .agent-state in whatever folder someone was standing in.
func TestDiscoveryReportsWhetherItFoundAWorkspace(t *testing.T) {
	bare := t.TempDir()
	chdir(t, bare)
	if _, found, err := discoverWorkspace(); err != nil {
		t.Fatalf("discoverWorkspace: %v", err)
	} else if found {
		t.Error("a directory with no marker reported a workspace")
	}

	marked := t.TempDir()
	if err := os.WriteFile(filepath.Join(marked, "AGENTS.md"), []byte("# rules\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	chdir(t, marked)
	root, found, err := discoverWorkspace()
	if err != nil {
		t.Fatalf("discoverWorkspace: %v", err)
	}
	if !found {
		t.Error("a directory with a marker did not report a workspace")
	}
	expectRoot(t, marked, root)
}

// Outside a workspace the cache goes somewhere shared, so a second fetch from a
// different folder hits it instead of starting again.
func TestTheFetchCacheLeavesNonWorkspacesAlone(t *testing.T) {
	bare := t.TempDir()

	outside := fetchCacheRoot(bare, false)
	if outside == bare {
		t.Error("a directory that is not a workspace was used as the cache root")
	}
	if cache, err := os.UserCacheDir(); err == nil {
		if !strings.HasPrefix(outside, cache) {
			t.Errorf("cache root = %q, want it under %q", outside, cache)
		}
	}

	// Two different bare directories must agree on where the cache lives, or it
	// is rebuilt per folder and the token budget pays for it every time.
	if second := fetchCacheRoot(t.TempDir(), false); second != outside {
		t.Errorf("two bare directories chose different caches: %q and %q", outside, second)
	}

	// Inside a workspace nothing changes.
	if inside := fetchCacheRoot(bare, true); inside != bare {
		t.Errorf("cache root = %q inside a workspace, want %q", inside, bare)
	}
}
