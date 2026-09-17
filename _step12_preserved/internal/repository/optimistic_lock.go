package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// ErrOptimisticLockConflict is returned by guarded repository updates when the
// caller's expected Version no longer matches the stored row. The caller must
// never interpret this as success: the write was not applied and no Version
// was incremented. Handlers surface it to the UI as a safe application-level
// conflict ("another user updated the record") instead of leaking database
// errors.
var ErrOptimisticLockConflict = errors.New("record was updated by another user")

// ErrInvalidRecordVersion is returned when an update request carries no usable
// expected version (missing, zero or negative). It is a client contract
// violation — the web forms must always submit the version they read — and is
// deliberately distinct from ErrOptimisticLockConflict so handlers can answer
// 400 Bad Request instead of 409 Conflict without inviting the user to reload
// (reloading cannot fix a request that never carried a version).
var ErrInvalidRecordVersion = errors.New("expected record version is required")

// applyOptimisticUpdate performs an atomic compare-and-swap update:
//
//	UPDATE ... SET <fields>, version = version + 1, updated_at = now
//	WHERE id = ? AND version = ? AND deleted_at IS NULL
//
// The expected version participates directly in the UPDATE predicate, so the
// check-and-write cannot be separated by a concurrent transaction (the race
// window of a SELECT-then-UPDATE is eliminated). The next version is computed
// by the database (version + 1) — a client can never push an arbitrary
// version value into the row. The deleted_at predicate keeps soft-deleted rows
// immutable: a stale submit against a deleted record can never resurrect it.
//
// On zero affected rows the method distinguishes "record missing" (notFound)
// from "version stale or soft-deleted" (ErrOptimisticLockConflict) with one
// existence probe that includes soft-deleted rows.
func applyOptimisticUpdate(
	db *gorm.DB,
	model any,
	table string,
	id any,
	expectedVersion int,
	fields map[string]any,
	notFound error,
) error {
	if expectedVersion < 1 {
		return ErrInvalidRecordVersion
	}

	// The next version is computed by the database, never taken from a client
	// value. The client only ever supplies the *expected* version used in the
	// WHERE clause.
	fields["version"] = gorm.Expr("version + 1")
	fields["updated_at"] = time.Now().UTC()

	result := db.Model(model).
		Where("id = ? AND version = ?", id, expectedVersion).
		Where("deleted_at IS NULL").
		Updates(fields)
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected > 0 {
		return nil
	}

	// Zero affected rows: the row is either gone (physically missing) or its
	// version has moved on (including soft-deleted rows, which the UPDATE
	// above refuses to touch). Classify with one existence probe that
	// includes soft-deleted rows so the caller gets a conflict — not a
	// not-found — for a stale submit against a deleted record.
	var row struct {
		Version int
	}
	err := db.Unscoped().Table(table).Select("version").Where("id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return notFound
	}
	if err != nil {
		return err
	}

	return ErrOptimisticLockConflict
}
