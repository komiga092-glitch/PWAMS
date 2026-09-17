package services

import "github.com/komiga092-glitch/pwams/internal/repository"

// Re-exports of the repository-level optimistic-locking sentinels. Services
// propagate these unchanged; handlers depend on the services package (not the
// repository package) to translate them into HTTP responses.
var (
	// ErrOptimisticLockConflict is returned when an update was rejected
	// because the record changed since the caller read it (or was
	// soft-deleted). The write was not applied; the caller must reload.
	ErrOptimisticLockConflict = repository.ErrOptimisticLockConflict

	// ErrInvalidRecordVersion is returned when an update request carries no
	// usable expected version. It is a client contract violation, not a
	// concurrency conflict.
	ErrInvalidRecordVersion = repository.ErrInvalidRecordVersion
)
