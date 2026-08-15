package tool

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/slopaudit/domain"
)

type fakeExecutor struct {
	missing bool
	exit    int
	output  string
}

func (f fakeExecutor) LookPath(file string) (string, error) {
	if f.missing {
		return "", errors.New("missing")
	}
	return file, nil
}

func (f fakeExecutor) Run(ctx context.Context, name string, args ...string) (int, string, error) {
	if f.exit != 0 {
		return f.exit, f.output, errors.New("failed")
	}
	return 0, f.output, nil
}

func TestAdapterSkipsMissingTool(t *testing.T) {
	adapter := Adapter{name: ToolSlopDetector, binary: ToolSlopDetector, args: slopDetectorArgs, executor: fakeExecutor{missing: true}}
	result := adapter.Run(context.Background(), ".")
	if result.Status != domain.AdapterSkipped || result.Reason != ReasonNotInstalled {
		t.Fatalf("result = %+v", result)
	}
}

func TestAdapterReportsFailure(t *testing.T) {
	adapter := Adapter{name: ToolSlopDetector, binary: ToolSlopDetector, args: slopDetectorArgs, executor: fakeExecutor{exit: 2, output: "bad config"}}
	result := adapter.Run(context.Background(), ".")
	if result.Status != domain.AdapterFailed || result.ExitCode != 2 || result.Reason != "bad config" {
		t.Fatalf("result = %+v", result)
	}
}

func TestFirstConfigSearchesParentDirectories(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, ".semgrep.yml")
	if err := os.WriteFile(config, []byte("rules: []"), 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "nested", "pkg")
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatal(err)
	}

	if got := firstConfig(target, semgrepConfigs); got != config {
		t.Fatalf("firstConfig() = %q, want %q", got, config)
	}
}
