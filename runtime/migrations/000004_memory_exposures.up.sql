-- Evidence-based memory reuse.
--
-- An exposure is one memory put in front of the model while a run was in
-- flight: retrieved by a hook or returned by a memory search. It is credited
-- (memories.used_count + 1, a "use" ledger line) only when that same run later
-- records a check the runtime itself verified as passing. Being retrieved is
-- not reuse; being retrieved and then followed by verified success is the
-- strongest signal available without asking the model to grade itself.
CREATE TABLE IF NOT EXISTS memory_exposures (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    memory_id   TEXT NOT NULL,
    run_id      TEXT NOT NULL,
    exposed_at  TEXT NOT NULL,
    credited_at TEXT
);

-- One open exposure per memory per run: re-injecting a memory every prompt
-- must not let it earn more than one use per verified success.
CREATE UNIQUE INDEX IF NOT EXISTS idx_memory_exposures_open
    ON memory_exposures(memory_id, run_id) WHERE credited_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_memory_exposures_run ON memory_exposures(run_id, credited_at);
