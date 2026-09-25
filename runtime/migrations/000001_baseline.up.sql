-- Baseline schema matching post-migrate-agent-only-machine memory.db tables.
-- Applied by golang-migrate (embed) on database.Open.
-- IF NOT EXISTS keeps a pre-migrate workspace (tables already present, no
-- schema_migrations row) from failing on the first Up.

CREATE TABLE IF NOT EXISTS runs (
    run_id            TEXT PRIMARY KEY,
    slug              TEXT NOT NULL,
    date              TEXT NOT NULL,
    version           INTEGER NOT NULL,
    graph_id          TEXT NOT NULL,
    current_node      TEXT NOT NULL,
    status            TEXT NOT NULL,
    iteration         INTEGER NOT NULL,
    max_transitions   INTEGER NOT NULL,
    token_budget      INTEGER NOT NULL DEFAULT 0,
    wallclock_seconds INTEGER NOT NULL DEFAULT 0,
    tokens_used       INTEGER NOT NULL DEFAULT 0,
    stopped_by        TEXT NOT NULL DEFAULT '',
    body              TEXT NOT NULL,
    created_by TEXT NOT NULL DEFAULT '',
    reviewed_by_agents TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_runs_slug_version ON runs(slug, version);

CREATE TABLE IF NOT EXISTS run_events (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id   TEXT NOT NULL REFERENCES runs(run_id),
    sequence INTEGER NOT NULL,
    type     TEXT NOT NULL,
    node     TEXT NOT NULL DEFAULT '',
    at       TEXT NOT NULL,
    payload  TEXT NOT NULL DEFAULT '',
    created_by TEXT NOT NULL DEFAULT '',
    reviewed_by_agents TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_run_events_run_sequence ON run_events(run_id, sequence);

CREATE TABLE IF NOT EXISTS task_lists (
    id      TEXT PRIMARY KEY,
    slug    TEXT NOT NULL,
    date    TEXT NOT NULL,
    version INTEGER NOT NULL,
    body    TEXT NOT NULL,
    created_by TEXT NOT NULL DEFAULT '',
    reviewed_by_agents TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS fetch_cache (
    key        TEXT PRIMARY KEY,
    source     TEXT NOT NULL,
    fetched_at TEXT NOT NULL,
    body       TEXT NOT NULL,
    created_by TEXT NOT NULL DEFAULT '',
    reviewed_by_agents TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS journal_entries (
    id      INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id  TEXT,
    type    TEXT NOT NULL,
    node    TEXT NOT NULL DEFAULT '',
    at      TEXT NOT NULL,
    payload TEXT NOT NULL DEFAULT '',
    created_by TEXT NOT NULL DEFAULT '',
    reviewed_by_agents TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sdd_cache (
    key           TEXT PRIMARY KEY,
    url           TEXT NOT NULL,
    prompt        TEXT NOT NULL DEFAULT '',
    etag          TEXT NOT NULL DEFAULT '',
    last_modified TEXT NOT NULL DEFAULT '',
    content       TEXT NOT NULL,
    fetched_at    INTEGER NOT NULL,
    created_by TEXT NOT NULL DEFAULT '',
    reviewed_by_agents TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS memories (
    id            TEXT PRIMARY KEY,
    workspace_id  TEXT NOT NULL,
    kind          TEXT NOT NULL,
    content       TEXT NOT NULL,
    tags          TEXT NOT NULL DEFAULT '',
    confidence    REAL NOT NULL,
    status        TEXT NOT NULL,
    source_type   TEXT NOT NULL,
    source_ref    TEXT,
    evidence      TEXT NOT NULL,
    supersedes_id TEXT,
    used_count    INTEGER NOT NULL DEFAULT 0,
    expires_at    TEXT,
    valid_from    TEXT NOT NULL DEFAULT '',
    valid_to      TEXT,
    created_by    TEXT NOT NULL DEFAULT '',
    reviewed_by_agents TEXT NOT NULL DEFAULT '',
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS memories_workspace_status ON memories(workspace_id, status, kind);

CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
    memory_id UNINDEXED,
    content,
    tags
);
