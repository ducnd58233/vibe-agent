// Package migrations holds golang-migrate SQL pairs for memory.db.
//
// The CLI applies them via `make migrate-up`; database.Open embeds and applies
// the same files so agent processes never CREATE TABLE in package code.
package migrations

import "embed"

// SQL is the ordered up/down pair set.
//
//go:embed *.sql
var SQL embed.FS
