-- Agent memory upgrade: everything an agent remembers lives in memory.db.
--
--   session_events  host gestures (was session.ndjson per run + ambient log),
--                   full-text indexed so past conversations are searchable
--                   episodic memory rather than an opaque replay file
--   agent_state     small keyed hook state (was cursor-node.json)
--   memory_links    typed edges between memories (supersedes, relates_to,
--                   derived_from, contradicts) for one-hop retrieval expansion
--   memory_events   append-only ledger of every memory mutation, so a record's
--                   provenance and status history survive later edits
--
-- memories_fts is rebuilt with the porter stemmer so "failing" matches "failed".

CREATE TABLE IF NOT EXISTS session_events (
    scope      TEXT NOT NULL,
    sequence   INTEGER NOT NULL,
    type       TEXT NOT NULL,
    at         TEXT NOT NULL,
    payload    TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    PRIMARY KEY (scope, sequence)
);

CREATE INDEX IF NOT EXISTS idx_session_events_at ON session_events(at);

CREATE VIRTUAL TABLE IF NOT EXISTS session_events_fts USING fts5(
    text,
    tokenize = 'porter unicode61'
);

-- Thinking rows are host reasoning that must not surface in recall.
CREATE TRIGGER IF NOT EXISTS session_events_ai AFTER INSERT ON session_events
WHEN new.type <> 'thinking' AND json_valid(new.payload)
BEGIN
    INSERT INTO session_events_fts (rowid, text)
    VALUES (new.rowid, trim(
        coalesce(json_extract(new.payload, '$.body'), '') || ' ' ||
        coalesce(json_extract(new.payload, '$.command'), '') || ' ' ||
        coalesce(json_extract(new.payload, '$.tool'), '')));
END;

CREATE TRIGGER IF NOT EXISTS session_events_ad AFTER DELETE ON session_events
BEGIN
    DELETE FROM session_events_fts WHERE rowid = old.rowid;
END;

CREATE TABLE IF NOT EXISTS agent_state (
    namespace  TEXT NOT NULL,
    key        TEXT NOT NULL,
    value      TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    PRIMARY KEY (namespace, key)
);

CREATE TABLE IF NOT EXISTS memory_links (
    src_id     TEXT NOT NULL,
    dst_id     TEXT NOT NULL,
    relation   TEXT NOT NULL,
    created_by TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    PRIMARY KEY (src_id, dst_id, relation)
);

CREATE INDEX IF NOT EXISTS idx_memory_links_dst ON memory_links(dst_id);

CREATE TABLE IF NOT EXISTS memory_events (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    memory_id   TEXT NOT NULL,
    action      TEXT NOT NULL,
    from_status TEXT NOT NULL DEFAULT '',
    to_status   TEXT NOT NULL DEFAULT '',
    actor       TEXT NOT NULL DEFAULT '',
    detail      TEXT NOT NULL DEFAULT '',
    at          TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_memory_events_memory ON memory_events(memory_id, id);

DROP TABLE IF EXISTS memories_fts;
CREATE VIRTUAL TABLE memories_fts USING fts5(
    memory_id UNINDEXED,
    content,
    tags,
    tokenize = 'porter unicode61'
);
INSERT INTO memories_fts (memory_id, content, tags)
    SELECT id, content, tags FROM memories;
