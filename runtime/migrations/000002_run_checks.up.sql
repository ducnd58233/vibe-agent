-- Child table for run verification checks (FINDINGS H1: stop rewriting checks
-- inside runs.body). Load merges these rows into Run.Checks; Save upserts here
-- and omits checks from the body JSON.

CREATE TABLE IF NOT EXISTS run_checks (
    run_id    TEXT NOT NULL REFERENCES runs(run_id),
    name      TEXT NOT NULL,
    passed    INTEGER NOT NULL,
    skipped   INTEGER NOT NULL DEFAULT 0,
    source    TEXT NOT NULL,
    ref       TEXT NOT NULL DEFAULT '',
    exit_code INTEGER,
    at        TEXT NOT NULL,
    PRIMARY KEY (run_id, name)
);

CREATE INDEX IF NOT EXISTS idx_run_checks_run_id ON run_checks(run_id);
