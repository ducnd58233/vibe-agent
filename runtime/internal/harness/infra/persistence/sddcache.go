package persistence

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/infra/database"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

// SDDCacheTable is the row store Python hooks and Go backfill share.
// DDL lives in runtime/migrations; keep the name in sync with the SQL and
// with .ai-agents/hooks/sdd-cache-*.py.
const SDDCacheTable = "sdd_cache"

// OpenSDDCache opens memory.db for sdd_cache access. Schema is applied by
// database.Open from embedded migrations.
func OpenSDDCache(ctx context.Context, workspaceRoot string) (*sql.DB, error) {
	path := workspace.MemoryDBPath(workspaceRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	return database.Open(ctx, path)
}
