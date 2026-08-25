package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/testutil"
)

func TestLoadMissingIsNotFound(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	_, found, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if found {
		t.Fatal("expected found=false")
	}
}

func TestLoadValidates(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeExample(t, root)
	cfg, found, err := Load(root)
	if err != nil || !found {
		t.Fatalf("Load: found=%v err=%v", found, err)
	}
	name, spec, err := cfg.ResolveRunner("unit", "")
	if err != nil {
		t.Fatal(err)
	}
	if name != "local" || spec.Driver != "local" {
		t.Fatalf("got %s %+v", name, spec)
	}
	name, spec, err = cfg.ResolveRunner("e2e", "")
	if err != nil {
		t.Fatal(err)
	}
	if name != "docker" || spec.Driver != "docker" {
		t.Fatalf("got %s %+v", name, spec)
	}
}

func TestLoadRejectsBadDriver(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := Path(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	bad := `apiVersion: vibe-agent/v1
kind: SandboxConfig
spec:
  runners:
    x:
      driver: firecracker
`
	if err := os.WriteFile(path, []byte(bad), 0o600); err != nil {
		t.Fatal(err)
	}
	_, found, err := Load(root)
	if !found || err == nil {
		t.Fatalf("expected validation error, found=%v err=%v", found, err)
	}
}

func TestLocalUpExecDown(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	testutil.EnsureRunIndex(t, root, "sandbox-demo")
	writeExample(t, root)

	ctx := context.Background()
	if err := Up(ctx, root, "sandbox-demo", "unit", ""); err != nil {
		t.Fatalf("Up: %v", err)
	}
	st, found, err := ReadStatus(root, "sandbox-demo", "unit")
	if err != nil || !found || st.State != StatusUp || st.Runner != "local" {
		t.Fatalf("status after up: found=%v st=%+v err=%v", found, st, err)
	}

	result := Exec(ctx, ExecRequest{
		WorkspaceRoot: root,
		Slug:          "sandbox-demo",
		UseCase:       "unit",
		Command:       "go",
		Args:          []string{"env", "GOVERSION"},
		Timeout:       time.Minute,
	})
	if result.NeverRan || result.ExitCode != 0 {
		t.Fatalf("Exec: %+v output=%s", result, result.Output)
	}
	if result.Driver != "local" {
		t.Fatalf("driver=%s", result.Driver)
	}

	if err := Down(root, "sandbox-demo", "unit"); err != nil {
		t.Fatalf("Down: %v", err)
	}
	st, found, err = ReadStatus(root, "sandbox-demo", "unit")
	if err != nil || !found || st.State != StatusDown {
		t.Fatalf("status after down: found=%v st=%+v err=%v", found, st, err)
	}
}

func TestExecFailsClosedWithoutConfig(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	testutil.EnsureRunIndex(t, root, "sandbox-demo")
	result := Exec(context.Background(), ExecRequest{
		WorkspaceRoot: root,
		Slug:          "sandbox-demo",
		UseCase:       "unit",
		Runner:        "local",
		Command:       "go",
		Args:          []string{"env", "GOVERSION"},
	})
	if !result.NeverRan || result.Err == nil {
		t.Fatalf("expected fail closed, got %+v", result)
	}
	if !strings.Contains(result.Err.Error(), "missing") && !strings.Contains(result.Err.Error(), "fail closed") {
		t.Fatalf("error should mention missing config: %v", result.Err)
	}
}

func writeExample(t *testing.T, root string) {
	t.Helper()
	path := Path(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(ExampleConfig()), 0o600); err != nil {
		t.Fatal(err)
	}
}
