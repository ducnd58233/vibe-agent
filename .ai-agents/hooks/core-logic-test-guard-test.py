"""Contract for core-logic-test-guard.py.

Run directly: python3 .ai-agents/hooks/core-logic-test-guard-test.py

The cases that must NOT be flagged matter more than the ones that must. This
guard warns on every write to a test file, so a false positive is noise on work
that was already correct, and noise is how a guard gets ignored. Three of the
negative cases are taken from this repository's own suite.
"""

from __future__ import annotations

import json
import subprocess
import sys
import tempfile
from pathlib import Path

HOOK = Path(__file__).resolve().parent / "core-logic-test-guard.py"


def run(filename: str, source: str) -> tuple[int, str]:
    """Write source to a temp file, fire the hook at it, return (code, stdout)."""
    with tempfile.TemporaryDirectory() as temp_dir:
        target = Path(temp_dir) / filename
        target.write_text(source, encoding="utf-8")
        payload = {
            "tool_name": "Write",
            "tool_input": {"file_path": str(target)},
        }
        proc = subprocess.run(
            [sys.executable, str(HOOK)],
            input=json.dumps(payload),
            text=True,
            capture_output=True,
        )
        return proc.returncode, proc.stdout


def flagged(filename: str, source: str) -> bool:
    code, out = run(filename, source)
    if code != 0:
        raise AssertionError(f"hook exited {code}; a guard must never fail the edit")
    return out.strip() != ""


FLAG_CASES: list[tuple[str, str, str]] = [
    (
        "go test with no assertion at all",
        "svc_test.go",
        """package svc

func TestCreateOrder(t *testing.T) {
	order := New("abc")
	_ = order.Total()
}
""",
    ),
    (
        "go test asserting only an environment variable",
        "env_test.go",
        """package svc

func TestDatabaseURLIsSet(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Fatal("DATABASE_URL is not set")
	}
}
""",
    ),
    (
        "go test named for container health",
        "infra_test.go",
        """package svc

func TestPostgresContainerIsUp(t *testing.T) {
	if err := db.Ping(); err != nil {
		t.Fatalf("postgres is not up: %v", err)
	}
}
""",
    ),
    (
        "go test whose name and only assertion are path discovery",
        "layout_test.go",
        """package svc

func TestConfigFileExists(t *testing.T) {
	if _, err := os.Stat("config.json"); err != nil {
		t.Fatal("config.json is missing")
	}
}
""",
    ),
    (
        "javascript test with no expect",
        "order.test.ts",
        """describe('createOrder', () => {
  it('creates an order', async () => {
    const order = await createOrder({ total: 10 });
    console.log(order);
  });
});
""",
    ),
    (
        "javascript test asserting only that a mock was called",
        "wrapper.test.ts",
        """describe('OrderGateway', () => {
  it('forwards to the repository', async () => {
    await gateway.save(order);
    expect(repository.save).toHaveBeenCalled();
  });
});
""",
    ),
    (
        "python test asserting only an environment variable",
        "test_config.py",
        """import os


def test_database_url_present():
    assert os.environ["DATABASE_URL"]
""",
    ),
]

QUIET_CASES: list[tuple[str, str, str]] = [
    (
        "ordinary go test with a real assertion",
        "svc_test.go",
        """package svc

func TestCreateOrderComputesTotal(t *testing.T) {
	order := New("abc")
	if got := order.Total(); got != 30 {
		t.Errorf("total is %d, want 30", got)
	}
}
""",
    ),
    (
        "writing a file IS the behavior, so observing it is not discovery",
        "run_test.go",
        """package state

func TestSavingARunWritesAManifestIntoTheWorkspace(t *testing.T) {
	root := t.TempDir()
	if err := Save(ManifestPath(root, "demo"), run); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := os.Stat(ManifestPath(root, "demo")); err != nil {
		t.Fatalf("state was not written into the workspace: %v", err)
	}
}
""",
    ),
    (
        "asserting the ABSENCE of a side effect is behavior",
        "journal_test.go",
        """package harness

func TestPostToolUseProposesNothingWithoutAFailure(t *testing.T) {
	root := workspaceWithRun(t)
	invoke(t, request(root))
	if _, err := os.Stat(memory.DBPath(root)); err == nil {
		t.Error("a passing command created a memory database")
	}
}
""",
    ),
    (
        "isolation is behavior even though it is observed on disk",
        "e2e_test.go",
        """package e2e

func TestEachWorkspaceGetsItsOwnDatabase(t *testing.T) {
	for _, root := range []string{first, second} {
		if _, err := os.Stat(filepath.Join(root, ".agent-state", "memory.db")); err != nil {
			t.Errorf("workspace %s has no database of its own: %v", root, err)
		}
	}
}
""",
    ),
    (
        "a mock assertion alongside a real one is not a wrapper test",
        "order.test.ts",
        """describe('createOrder', () => {
  it('persists the order and returns its total', async () => {
    const order = await createOrder({ items });
    expect(order.total).toBe(30);
    expect(repository.save).toHaveBeenCalled();
  });
});
""",
    ),
    (
        "a skipped test is not an assertion-free test",
        "pending_test.go",
        """package svc

func TestRefundFlow(t *testing.T) {
	t.Skip("refunds land next sprint")
}
""",
    ),
    (
        "the allow marker silences a deliberate case",
        "layout_test.go",
        """package svc

// core-logic-test-guard: allow
func TestConfigFileExists(t *testing.T) {
	if _, err := os.Stat("config.json"); err != nil {
		t.Fatal("config.json is missing")
	}
}
""",
    ),
    (
        "a non-test file is none of this guard's business",
        "main.go",
        """package main

func main() {
	run()
}
""",
    ),
]


def main() -> int:
    failures: list[str] = []

    for label, filename, source in FLAG_CASES:
        if not flagged(filename, source):
            failures.append(f"NOT FLAGGED but should be: {label}")

    for label, filename, source in QUIET_CASES:
        if flagged(filename, source):
            code, out = run(filename, source)
            failures.append(f"FALSE POSITIVE: {label}\n    {out.strip()[:300]}")

    if failures:
        print(f"core-logic-test-guard-test: {len(failures)} failure(s)")
        for failure in failures:
            print(f"  - {failure}")
        return 1

    total = len(FLAG_CASES) + len(QUIET_CASES)
    print(f"core-logic-test-guard-test: OK ({total} cases)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
