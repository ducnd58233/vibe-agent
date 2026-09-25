package harness

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	for _, name := range []string{"sdd-cache-pre.py", "sdd-cache-post.py"} {
		raw, err := os.ReadFile(filepath.Clean(filepath.Join(toolkitRoot, ".ai-agents", "hooks", name)))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		source := string(raw)

		if !strings.Contains(source, workspace.EnvMemoryDBPath) {
			t.Errorf("%s does not read %s, so the runtime cannot place its database",
				name, workspace.EnvMemoryDBPath)
		}
		for _, host := range []string{".claude", ".cursor", ".codex", ".opencode"} {
			if strings.Contains(source, host+"/sdd-cache") || strings.Contains(source, `"`+host+`"`) {
				t.Errorf("%s still names %s for the cache; derived state has one home", name, host)
			}
		}
		if !strings.Contains(source, workspace.StateDirName) {
			t.Errorf("%s has no %s fallback for a standalone run", name, workspace.StateDirName)
		}
		if !strings.Contains(source, "sqlite3") {
			t.Errorf("%s does not open the database directly, so it is still file-based", name)
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
	if err := createSDDCacheTableForTest(ctx, db); err != nil {
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

	script := `
import sqlite3, sys
conn = sqlite3.connect(sys.argv[1], timeout=5)
conn.execute("PRAGMA busy_timeout = 5000")
conn.execute("PRAGMA journal_mode = WAL")
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
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	// Give the subprocess time to actually attempt the write and start
	// waiting on the lock before this releases it.
	time.Sleep(100 * time.Millisecond)
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

func createSDDCacheTableForTest(ctx context.Context, db *sql.DB) error {
	return database.CreateTableWithProvenance(ctx, db, "sdd_cache", `
        key           TEXT PRIMARY KEY,
        url           TEXT NOT NULL,
        prompt        TEXT NOT NULL DEFAULT '',
        etag          TEXT NOT NULL DEFAULT '',
        last_modified TEXT NOT NULL DEFAULT '',
        content       TEXT NOT NULL,
        fetched_at    INTEGER NOT NULL`)
}

func pythonInterpreter(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("python3"); err == nil {
		return "python3"
	}
	return "python"
}
