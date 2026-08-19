package database

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

const Driver = "sqlite"

// Open opens a SQLite database at path.
func Open(_ context.Context, path string) (*sql.DB, error) {
	db, err := sql.Open(Driver, path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	return db, nil
}
