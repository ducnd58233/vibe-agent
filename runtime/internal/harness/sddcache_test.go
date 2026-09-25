package harness

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/infra/database"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

// The WebFetch cache pair stayed in Python and each host config named it by a
// relative path. Only Claude Code documents the directory that resolves
// against, so on Cursor and Codex the interpreter was handed a path from an
// unknown starting point and the cache silently never ran.

// A tool that is not WebFetch must not pay for the cache at all. Spawning an
// interpreter on every Bash call would put a process launch in the hot path of
// the most frequent tool there is.
func TestSDDCacheIgnoresEveryOtherTool(t *testing.T) {
	blocked := sddCache(Request{
		WorkspaceRoot: t.TempDir(), ToolkitRoot: toolkitRoot, Client: ClientClaude,
	}, payload{ToolName: "Bash"}, "sdd-cache-pre.py")

	if blocked != nil {
		t.Errorf("a Bash call was sent through the WebFetch cache: %v", blocked)
	}
}

// A missing script is not a broken session. The cache is an optimisation, and a
// hook that fails because an optional accelerator is absent is worse than one
// that quietly does not accelerate.
func TestSDDCacheIsSilentWhenTheScriptIsAbsent(t *testing.T) {
	blocked := sddCache(Request{
		WorkspaceRoot: t.TempDir(), ToolkitRoot: t.TempDir(), Client: ClientClaude,
	}, payload{ToolName: "WebFetch"}, "sdd-cache-pre.py")

	if blocked != nil {
		t.Errorf("an absent script refused a tool call: %v", blocked)
	}
}

// A refusal from the cache is a refusal on the same event as the safety gate,
// so it has to leave through the same door. It did not at first: the block was
// returned past the per-host translation, and Cursor and Codex received exit 0
// and an empty reply while Claude got the cached page.
func TestACacheRefusalIsDeliveredInEachHostsShape(t *testing.T) {
	reason := &BlockError{Reason: "[sdd-cache] Cache hit"}

	for _, host := range []struct {
		client Client
		expect string
	}{
		{ClientCursor, "permission"},
		{ClientCodex, "permissionDecision"},
	} {
		var out bytes.Buffer
		if err := deliverBlock(Request{Client: host.client}, reason, &out); err != nil {
			t.Fatalf("%s: %v", host.client, err)
		}
		if !strings.Contains(out.String(), host.expect) {
			t.Errorf("%s got no refusal it can read: %s", host.client, out.String())
		}
	}

	// Claude decides by exit status, so its refusal is the returned error and
	// main turns it into exit 2 with the reason on stderr.
	var out bytes.Buffer
	if err := deliverBlock(Request{Client: ClientClaude}, reason, &out); err == nil {
		t.Error("Claude got no error to exit 2 with")
	}
}

// The scripts cannot import the Go constant, so the runtime hands them the
// resolved database path. This is asserted against the script source rather
// than by running them, because both make a network call before touching the
// cache and a test that needs the network is a test that gets skipped.
//
// The bug this guards: both scripts hardcoded .claude/sdd-cache, so a Cursor or
// opencode session wrote its cache into another host's directory. Nothing
// failed, nothing was reported, and the cache looked like it was working.
func TestTheCacheScriptsTakeTheirDatabasePathFromTheRuntime(t *testing.T) {
	hooks := filepath.Join(toolkitRoot, ".ai-agents", "hooks")

	commonRaw, err := os.ReadFile(filepath.Clean(filepath.Join(hooks, "sdd-cache-common.py")))
	if err != nil {
		t.Fatalf("sdd-cache-common.py: %v", err)
	}
	common := string(commonRaw)
	if !strings.Contains(common, workspace.EnvMemoryDBPath) {
		t.Errorf("sdd-cache-common.py does not read %s, so the runtime cannot place its database",
			workspace.EnvMemoryDBPath)
	}
	for _, host := range []string{".claude", ".cursor", ".codex", ".opencode"} {
		if strings.Contains(common, host+"/sdd-cache") || strings.Contains(common, `"`+host+`"`) {
			t.Errorf("sdd-cache-common.py still names %s for the cache; derived state has one home", host)
		}
	}
	if !strings.Contains(common, workspace.StateDirName) {
		t.Errorf("sdd-cache-common.py has no %s fallback for a standalone run", workspace.StateDirName)
	}
	if !strings.Contains(common, "sqlite3") {
		t.Error("sdd-cache-common.py does not open the database directly, so it is still file-based")
	}

	for _, name := range []string{"sdd-cache-pre.py", "sdd-cache-post.py"} {
		raw, err := os.ReadFile(filepath.Clean(filepath.Join(hooks, name)))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		source := string(raw)
		if !strings.Contains(source, "sdd-cache-common.py") {
			t.Errorf("%s does not load sdd-cache-common.py, so it may carry its own copy of the path logic", name)
		}
		for _, host := range []string{".claude", ".cursor", ".codex", ".opencode"} {
			if strings.Contains(source, host+"/sdd-cache") || strings.Contains(source, `"`+host+`"`) {
				t.Errorf("%s still names %s for the cache; derived state has one home", name, host)
			}
		}
	}
}

// Two runtimes open the same file now: Go for run state and memory, Python
// for sdd_cache. This is the one place that actually happens, so it is the
// one place WAL + busy_timeout (T1) has to be proven, not assumed, to cover.
//
// Go holds a write transaction open; a Python subprocess inserts a row on the
// same file concurrently. Without WAL/busy_timeout the Python write fails
// immediately with "database is locked"; with them it waits for Go to commit.
// This repository's own CI already requires python3 (check-schemas.py,
// check-frontmatter.py), so this test does not guard against its absence.
func TestAConcurrentPythonWriteWaitsOnAGoTransaction(t *testing.T) {
	root := t.TempDir()
	dbPath := workspace.MemoryDBPath(root)
	ctx := context.Background()

	if err := os.MkdirAll(filepath.Dir(dbPath), 0o750); err != nil {
		t.Fatal(err)
	}
	db, err := database.Open(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	})
	if err := createSDDCacheTable(ctx, db); err != nil {
		t.Fatal(err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `
        INSERT INTO sdd_cache (key, url, content, fetched_at, created_at, updated_at)
        VALUES ('go-held', 'https://example.com/go', 'x', 0, '', '')`); err != nil {
		t.Fatal(err)
	}

	// Prints "ready" the instant the connection and pragmas are set, right
	// before attempting the write - Go blocks on that line instead of
	// guessing how long a subprocess start takes, which a fixed sleep would.
	script := `
import sqlite3, sys
conn = sqlite3.connect(sys.argv[1], timeout=5)
conn.execute("PRAGMA busy_timeout = 5000")
conn.execute("PRAGMA journal_mode = WAL")
print("ready", flush=True)
conn.execute(
    "INSERT INTO sdd_cache (key, url, content, fetched_at, created_at, updated_at) "
    "VALUES ('py-written', 'https://example.com/py', 'y', 0, '', '')"
)
conn.commit()
conn.close()
`
	cmd := exec.CommandContext(ctx, pythonInterpreter(t), "-c", script, dbPath) //nolint:gosec // G204: script is a fixed literal above; dbPath is this test's own t.TempDir() path, not external input. vibe-agent: allow-suppression
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	ready, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || strings.TrimSpace(ready) != "ready" {
		t.Fatalf("python did not signal readiness: %q, %v\n%s", ready, err, stderr.String())
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	if err := cmd.Wait(); err != nil {
		t.Errorf("python write was refused instead of waiting: %v\n%s", err, stderr.String())
	}

	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM sdd_cache WHERE key = 'py-written'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("the python-written row is missing after the wait")
	}
}

// A table Go creates through the real production schema (createSDDCacheTable,
// not a copy in this test file) must be readable through the real hook
// module's own read shape - not just through an inline script that happens to
// match by coincidence. This is the direction TestBackfillMovesEveryExisting
// FileThenDeletesIt (fetch's) and the Python-only sdd-cache-test.py contract
// check do not cover: Go writes, Python's own code reads.
func TestAGoCreatedRowIsReadableThroughThePythonHookModule(t *testing.T) {
	root := t.TempDir()
	dbPath := workspace.MemoryDBPath(root)
	ctx := context.Background()

	if err := os.MkdirAll(filepath.Dir(dbPath), 0o750); err != nil {
		t.Fatal(err)
	}
	db, err := database.Open(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	})
	if err := createSDDCacheTable(ctx, db); err != nil {
		t.Fatal(err)
	}
	url := "https://example.com/go-row"
	sum := sha256.Sum256([]byte(url))
	key := hex.EncodeToString(sum[:])[:32]
	if _, err := db.ExecContext(ctx, `
        INSERT INTO sdd_cache (key, url, prompt, etag, last_modified, content, fetched_at, created_at, updated_at)
        VALUES (?, ?, 'go prompt', 'W/"g"', '', 'go content', 0, '', '')`, key, url); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	script := `
import importlib.util, sys

spec = importlib.util.spec_from_file_location("sdd_cache_common", sys.argv[2])
common = importlib.util.module_from_spec(spec)
spec.loader.exec_module(common)

import os
os.environ["VIBE_MEMORY_DB_PATH"] = sys.argv[1]
conn = common.open_cache_db()
row = conn.execute(
    "SELECT prompt, etag, content FROM sdd_cache WHERE key = ?",
    (common.cache_key_for_url("https://example.com/go-row"),),
).fetchone()
conn.close()
print("MATCH" if row == ("go prompt", 'W/"g"', "go content") else f"MISMATCH {row!r}")
`
	commonPath := filepath.Join(toolkitRoot, ".ai-agents", "hooks", "sdd-cache-common.py")
	//nolint:gosec // G204: script is a fixed literal above; dbPath and commonPath are this test's own paths, not external input. vibe-agent: allow-suppression
	out, err := exec.CommandContext(ctx, pythonInterpreter(t), "-c", script, dbPath, commonPath).Output()
	if err != nil {
		t.Fatalf("python read: %v", err)
	}
	if strings.TrimSpace(string(out)) != "MATCH" {
		t.Errorf("python's own read shape did not find the Go-written row: %s", out)
	}
}

func pythonInterpreter(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("python3"); err == nil {
		return "python3"
	}
	return "python"
}
