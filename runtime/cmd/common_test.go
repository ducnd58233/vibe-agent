package main

import (
	"os"
	"path/filepath"
	"runtime"
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
func TestHoldsAssetsRejectsAFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".ai-agents"), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if holdsAssets(dir) {
		t.Fatal("a file named .ai-agents was accepted as a toolkit root")
	}
}
