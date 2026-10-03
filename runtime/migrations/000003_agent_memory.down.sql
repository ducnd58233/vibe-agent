DROP TABLE IF EXISTS memories_fts;
CREATE VIRTUAL TABLE memories_fts USING fts5(
    memory_id UNINDEXED,
    content,
    tags
);
INSERT INTO memories_fts (memory_id, content, tags)
    SELECT id, content, tags FROM memories;
DROP INDEX IF EXISTS idx_memory_events_memory;
DROP TABLE IF EXISTS memory_events;
DROP INDEX IF EXISTS idx_memory_links_dst;
DROP TABLE IF EXISTS memory_links;
DROP TABLE IF EXISTS agent_state;
DROP TRIGGER IF EXISTS session_events_ad;
DROP TRIGGER IF EXISTS session_events_ai;
DROP TABLE IF EXISTS session_events_fts;
DROP INDEX IF EXISTS idx_session_events_at;
DROP TABLE IF EXISTS session_events;
