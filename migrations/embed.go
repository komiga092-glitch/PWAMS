package migrations

import "embed"

// The SQL migration files are the single source of truth for the database
// schema. They are embedded so the tracked migration runner (see
// internal/database/migrator.go and cmd/migrate) works from a compiled
// binary without depending on the working directory at runtime.
//
// Every migration must be written idempotently (CREATE ... IF NOT EXISTS,
// guarded ALTERs) so applying it to a database whose structure was
// previously managed by GORM AutoMigrate is always safe.
//
//go:embed *.up.sql *.down.sql
var FS embed.FS
