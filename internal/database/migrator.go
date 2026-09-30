package database

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/komiga092-glitch/pwams/migrations"
	"gorm.io/gorm"
)

// This file implements the tracked migration architecture (PHASE 4C/4D):
//
//   - Every SQL file under /migrations is versioned (NNNNNN_name.up.sql) and
//     checksummed (SHA-256).
//   - Applied versions are recorded in a `schema_migrations` table created in
//     the canonical application schema (the first writable schema of the
//     connection's search_path — `pwams_user` in the current deployment).
//   - `MigrateUp` applies pending migrations in order, one transaction each,
//     and refuses to run when an already-applied file's checksum changed.
//   - `VerifyUpToDate` performs NO schema writes: production startup calls it
//     and fails fast when pending migrations exist, so the application can
//     never silently alter the schema at startup (PHASE 4D decision B).
//
// All shipped migration files are idempotent, so applying them to a database
// previously managed by GORM AutoMigrate is safe and never removes data.

// SchemaMigration is the tracking record for one applied migration.
type SchemaMigration struct {
	Version   int64     `gorm:"primaryKey;column:version"`
	Name      string    `gorm:"column:name;not null"`
	Checksum  string    `gorm:"column:checksum;size:64;not null"`
	AppliedAt time.Time `gorm:"column:applied_at;not null"`
}

// TableName pins the tracking table (never schema-qualified: it must be
// created in whatever schema current_schema() resolves to, i.e. the canonical
// application schema).
func (SchemaMigration) TableName() string { return "schema_migrations" }

// MigrationFile describes one embedded migration.
type MigrationFile struct {
	Version  int64
	Name     string
	UpSQL    string
	DownSQL  string
	Checksum string
}

// MigrationStatus is one row of the Status report.
type MigrationStatus struct {
	Version       int64
	Name          string
	Applied       bool
	ChecksumMatch bool
	AppliedAt     *time.Time
}

// LoadMigrationFiles reads and orders every embedded migration file.
func LoadMigrationFiles() ([]MigrationFile, error) {
	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded migrations: %w", err)
	}

	ups := map[int64]MigrationFile{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		var direction string
		var fileBase string

		switch {
		case strings.HasSuffix(name, ".up.sql"):
			direction = "up"
			fileBase = strings.TrimSuffix(name, ".up.sql")
		case strings.HasSuffix(name, ".down.sql"):
			direction = "down"
			fileBase = strings.TrimSuffix(name, ".down.sql")
		default:
			continue
		}

		parts := strings.SplitN(fileBase, "_", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("migration file %q does not match NNNNNN_name.up|.down.sql", name)
		}

		version, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("migration file %q has an invalid version prefix: %w", name, err)
		}

		content, err := fs.ReadFile(migrations.FS, name)
		if err != nil {
			return nil, fmt.Errorf("failed to read migration %q: %w", name, err)
		}

		file := ups[version]
		file.Version = version
		file.Name = parts[1]
		if direction == "up" {
			file.UpSQL = string(content)
			sum := sha256.Sum256(content)
			file.Checksum = hex.EncodeToString(sum[:])
		} else {
			file.DownSQL = string(content)
		}
		ups[version] = file
	}

	ordered := make([]MigrationFile, 0, len(ups))
	for _, file := range ups {
		if file.UpSQL == "" {
			return nil, fmt.Errorf("migration %06d_%s has no .up.sql file", file.Version, file.Name)
		}
		ordered = append(ordered, file)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Version < ordered[j].Version })

	return ordered, nil
}

// EnsureMigrationTable creates the tracking table in the canonical schema.
func EnsureMigrationTable(db *gorm.DB) error {
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version BIGINT PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			checksum VARCHAR(64) NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`).Error; err != nil {
		return fmt.Errorf("failed to ensure schema_migrations tracking table: %w", err)
	}
	return nil
}

// HasMigrationTable reports whether the tracking table exists. Read-only:
// it never creates the table.
func HasMigrationTable(db *gorm.DB) bool {
	return db.Migrator().HasTable("schema_migrations")
}

// AppliedMigrationsReadOnly returns the tracked records WITHOUT creating the
// tracking table; on a never-migrated database it returns an empty map. This
// is the only lookup used by read-only paths (status, verify, production
// startup verification).
func AppliedMigrationsReadOnly(db *gorm.DB) (map[int64]SchemaMigration, error) {
	out := map[int64]SchemaMigration{}
	if !HasMigrationTable(db) {
		return out, nil
	}

	var applied []SchemaMigration
	if err := db.Find(&applied).Error; err != nil {
		return nil, fmt.Errorf("failed to read schema_migrations: %w", err)
	}

	for _, record := range applied {
		out[record.Version] = record
	}
	return out, nil
}

// AppliedMigrations ensures the tracking table exists and returns the tracked
// records keyed by version. It is the WRITE-side lookup used only by
// MigrateUp (explicit `cmd/migrate up` runs).
func AppliedMigrations(db *gorm.DB) (map[int64]SchemaMigration, error) {
	if err := EnsureMigrationTable(db); err != nil {
		return nil, err
	}

	var applied []SchemaMigration
	if err := db.Find(&applied).Error; err != nil {
		return nil, fmt.Errorf("failed to read schema_migrations: %w", err)
	}

	out := make(map[int64]SchemaMigration, len(applied))
	for _, record := range applied {
		out[record.Version] = record
	}
	return out, nil
}

// Status reports every known migration and whether it is applied.
func Status(db *gorm.DB) ([]MigrationStatus, error) {
	files, err := LoadMigrationFiles()
	if err != nil {
		return nil, err
	}

	// Read-only lookup: Status (and therefore VerifyUpToDate and the
	// `status`/`verify` CLI commands) must never create the tracking table
	// and must never write anything.
	applied, err := AppliedMigrationsReadOnly(db)
	if err != nil {
		return nil, err
	}

	statuses := make([]MigrationStatus, 0, len(files))
	for _, file := range files {
		state := MigrationStatus{Version: file.Version, Name: file.Name, ChecksumMatch: true}
		if record, ok := applied[file.Version]; ok {
			state.Applied = true
			state.AppliedAt = &record.AppliedAt
			state.ChecksumMatch = record.Checksum == file.Checksum
		}
		statuses = append(statuses, state)
	}

	return statuses, nil
}

// VerifyUpToDate fails when pending migrations exist or when an applied
// migration's checksum no longer matches its file. It performs no writes.
func VerifyUpToDate(db *gorm.DB) error {
	statuses, err := Status(db)
	if err != nil {
		return err
	}

	var pending []string
	var drifted []string
	for _, state := range statuses {
		if !state.Applied {
			pending = append(pending, fmt.Sprintf("%06d_%s", state.Version, state.Name))
			continue
		}
		if !state.ChecksumMatch {
			drifted = append(drifted, fmt.Sprintf("%06d_%s", state.Version, state.Name))
		}
	}

	if len(drifted) > 0 {
		return fmt.Errorf(
			"applied migrations were modified after being applied: %s; restore the original files or re-baseline the database deliberately",
			strings.Join(drifted, ", "),
		)
	}
	if len(pending) > 0 {
		return fmt.Errorf(
			"database schema is out of date, pending migrations: %s; run 'go run ./cmd/migrate up' (with a verified backup) before starting the server",
			strings.Join(pending, ", "),
		)
	}
	return nil
}

// MigrateUp applies every pending migration in version order. Each migration
// runs in its own transaction together with its tracking record so a failure
// never leaves an applied-but-unrecorded (or recorded-but-unapplied) state.
func MigrateUp(db *gorm.DB) error {
	return MigrateUpTo(db, math.MaxInt64)
}

// MigrateUpTo applies pending migrations in version order, stopping after
// maxVersion (inclusive). It supports staged deployments and the migration
// test suite (clean baseline -> representative seed -> tenancy migrations).
func MigrateUpTo(db *gorm.DB, maxVersion int64) error {
	files, err := LoadMigrationFiles()
	if err != nil {
		return err
	}

	applied, err := AppliedMigrations(db)
	if err != nil {
		return err
	}

	for _, file := range files {
		if file.Version > maxVersion {
			break
		}
		if record, ok := applied[file.Version]; ok {
			if record.Checksum != file.Checksum {
				return fmt.Errorf(
					"migration %06d_%s was modified after being applied (checksum mismatch): refusing to continue",
					file.Version, file.Name,
				)
			}
			continue
		}

		err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec(file.UpSQL).Error; err != nil {
				return fmt.Errorf("migration %06d_%s failed: %w", file.Version, file.Name, err)
			}
			record := SchemaMigration{
				Version:   file.Version,
				Name:      file.Name,
				Checksum:  file.Checksum,
				AppliedAt: time.Now().UTC(),
			}
			if err := tx.Create(&record).Error; err != nil {
				return fmt.Errorf("failed to record migration %06d_%s: %w", file.Version, file.Name, err)
			}
			return nil
		})
		if err != nil {
			return err
		}

		applied[file.Version] = SchemaMigration{Version: file.Version, Checksum: file.Checksum}
	}

	return nil
}
