package repomap

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ducnd58233/vibe-agent/runtime/internal/memory"

	_ "modernc.org/sqlite"
)

// cacheVersion is what this build makes of a file's contents.
//
// Bump it whenever extraction changes. The content hash cannot catch that case:
// the file is byte-identical and what this package now reports about it is not,
// so every cached row is stale while every hash still matches. Aider learned the
// same lesson and calls it CACHE_VERSION.
const cacheVersion = 1

// CachePath is where a workspace's index lives.
//
// Beside the memory database, under the same gitignored state directory, for the
// same reason: it is derived from this checkout and belongs to it. Nothing here
// is ever written to the toolkit or to the user's home.
func CachePath(workspaceRoot string) string {
	return filepath.Join(workspaceRoot, memory.StateDirName, "repomap.db")
}

// Stat reports how many files the index holds, without building one.
//
// Non-creating on purpose, so a diagnostic can ask about the index in a
// workspace that has never built one and leave nothing behind. The same rule
// internal/memory follows for the same reason: a read that seeds a database in
// an unrelated directory is a read with a side effect.
func Stat(ctx context.Context, workspaceRoot string) (files int, exists bool, err error) {
	path := CachePath(workspaceRoot)
	if _, err := os.Stat(path); err != nil {
		return 0, false, nil
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return 0, true, fmt.Errorf("open %s: %w", path, err)
	}
	defer db.Close()
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM files`).Scan(&files); err != nil {
		return 0, true, fmt.Errorf("read %s: %w", path, err)
	}
	return files, true, nil
}

// cache is the per-file index, keyed by path and content hash.
type cache struct {
	db *sql.DB
}

const cacheSchema = `
CREATE TABLE IF NOT EXISTS files (
	path       TEXT PRIMARY KEY,
	hash       TEXT NOT NULL,
	symbols    TEXT NOT NULL,
	references_ TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS meta (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);
`

// openCache opens the index, discarding it whole when this build reads code
// differently than the build that wrote it.
func openCache(ctx context.Context, workspaceRoot string, version int) (*cache, error) {
	path := CachePath(workspaceRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create state directory: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	if _, err := db.ExecContext(ctx, cacheSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}

	c := &cache{db: db}
	stored, err := c.version(ctx)
	if err != nil {
		db.Close()
		return nil, err
	}
	if stored != version {
		if _, err := db.ExecContext(ctx, `DELETE FROM files`); err != nil {
			db.Close()
			return nil, fmt.Errorf("discard stale index: %w", err)
		}
		if _, err := db.ExecContext(ctx,
			`INSERT INTO meta (key, value) VALUES ('version', ?)
			 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
			fmt.Sprint(version)); err != nil {
			db.Close()
			return nil, fmt.Errorf("record index version: %w", err)
		}
	}
	return c, nil
}

func (c *cache) version(ctx context.Context) (int, error) {
	var raw string
	err := c.db.QueryRowContext(ctx, `SELECT value FROM meta WHERE key = 'version'`).Scan(&raw)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read index version: %w", err)
	}
	var version int
	if _, err := fmt.Sscanf(raw, "%d", &version); err != nil {
		return 0, nil
	}
	return version, nil
}

// entry is one file's cached index.
type entry struct {
	Symbols    []Symbol
	References []string
}

// lookup returns a file's index when the cached row was built from exactly this
// content.
func (c *cache) lookup(ctx context.Context, path, hash string) (entry, bool) {
	var storedHash, symbols, refs string
	err := c.db.QueryRowContext(ctx,
		`SELECT hash, symbols, references_ FROM files WHERE path = ?`, path,
	).Scan(&storedHash, &symbols, &refs)
	if err != nil || storedHash != hash {
		return entry{}, false
	}
	var found entry
	if json.Unmarshal([]byte(symbols), &found.Symbols) != nil {
		return entry{}, false
	}
	if json.Unmarshal([]byte(refs), &found.References) != nil {
		return entry{}, false
	}
	return found, true
}

func (c *cache) store(ctx context.Context, path, hash string, found entry) error {
	symbols, err := json.Marshal(found.Symbols)
	if err != nil {
		return err
	}
	refs, err := json.Marshal(found.References)
	if err != nil {
		return err
	}
	_, err = c.db.ExecContext(ctx,
		`INSERT INTO files (path, hash, symbols, references_) VALUES (?, ?, ?, ?)
		 ON CONFLICT(path) DO UPDATE SET
		     hash = excluded.hash,
		     symbols = excluded.symbols,
		     references_ = excluded.references_`,
		path, hash, string(symbols), string(refs))
	return err
}

// prune drops rows for files that are no longer in the workspace.
//
// Without it the map keeps sending a reader to a path that was deleted, which is
// the one failure that makes an index worse than no index at all.
func (c *cache) prune(ctx context.Context, live map[string]bool) error {
	rows, err := c.db.QueryContext(ctx, `SELECT path FROM files`)
	if err != nil {
		return err
	}
	var gone []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			rows.Close()
			return err
		}
		if !live[path] {
			gone = append(gone, path)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, path := range gone {
		if _, err := c.db.ExecContext(ctx, `DELETE FROM files WHERE path = ?`, path); err != nil {
			return err
		}
	}
	return nil
}

func (c *cache) Close() error { return c.db.Close() }
