package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ducnd58233/vibe-agent/runtime/migrations"
	"github.com/golang-migrate/migrate/v4"
	migrateSqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// Up applies every pending embedded migration to db. Safe to call on every
// Open: ErrNoChange is success. Baseline SQL uses IF NOT EXISTS so a
// pre-migrate workspace that already has tables still advances the ledger.
func Up(ctx context.Context, db *sql.DB) error {
	_ = ctx
	src, err := iofs.New(migrations.SQL, ".")
	if err != nil {
		return fmt.Errorf("open embedded migrations: %w", err)
	}
	driver, err := migrateSqlite.WithInstance(db, &migrateSqlite.Config{})
	if err != nil {
		return fmt.Errorf("sqlite migrate driver: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", src, Driver, driver)
	if err != nil {
		return fmt.Errorf("migrate instance: %w", err)
	}
	// Do not Close(m): migrate's sqlite driver Close also closes *sql.DB.
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}
