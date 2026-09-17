package services

import (
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/komiga092-glitch/pwams/internal/models"
)

// ErrRecordAccessDenied is returned when an authenticated principal
// attempts to access or mutate a record it does not own and holds no
// privileged role. Mapped to HTTP 403 by the handlers.
var ErrRecordAccessDenied = errors.New(
	"you do not have permission to access this record",
)

// Actor is the minimal authorization context extracted from the
// authenticated session and passed into service methods so ownership
// rules are enforced at the service layer (defense in depth below the
// HTTP RBAC middleware).
type Actor struct {
	ID   uuid.UUID
	Role string
}

// ActorFromUser builds an Actor from an authenticated user. Returns
// false when the user is nil or has no usable identity.
func ActorFromUser(user *models.User) (Actor, bool) {
	if user == nil || user.ID == uuid.Nil {
		return Actor{}, false
	}
	return Actor{ID: user.ID, Role: user.Role.Name}, true
}

// privilegedRoles lists the roles whose business function includes
// working with records created by other users (registration of
// beneficiaries, review of aid/loans, care delivery). This mirrors the
// existing RBAC permission matrix; non-privileged roles (Donor,
// Beneficiary, Student) are restricted to their own records. Keys are
// lowercase because IsPrivilegedRole normalizes input before lookup.
var privilegedRoles = map[string]bool{
	strings.ToLower(models.RoleSuperAdmin): true,
	strings.ToLower(models.RoleAdmin):      true,
	strings.ToLower(models.RoleManager):    true,
	strings.ToLower(models.RoleStaff):      true,
	strings.ToLower(models.RoleVolunteer):  true,
}

// IsPrivilegedRole reports whether the given role may operate on
// records it did not create.
func IsPrivilegedRole(roleName string) bool {
	return privilegedRoles[strings.ToLower(strings.TrimSpace(roleName))]
}

// CanAccessRecord reports whether the actor may read or mutate a
// record owned by ownerID. Privileged roles may access any record;
// everyone else is restricted to their own records.
func CanAccessRecord(actor Actor, ownerID uuid.UUID) bool {
	if actor.ID == uuid.Nil {
		return false
	}
	if IsPrivilegedRole(actor.Role) {
		return true
	}
	return actor.ID == ownerID
}

// ownershipFilter returns the owner ID that a list query must be scoped
// to, and reports whether the actor may list at all. Privileged roles
// receive uuid.Nil (meaning "no restriction" — they may see every
// record). Non-privileged roles receive their own actor ID so the
// repository query is filtered to records they own.
// A nil actor (ID == uuid.Nil) never receives unrestricted access:
// ok=false forces every caller to fail closed so an authorization
// decision can never be made for an unauthenticated principal. This
// closes the fail-open nil-actor behavior flagged during STEP 15.6.3
// (the previous report claimed fail-closed; the code was fail-open).
// This keeps authorization decisions in the service layer while the
// actual row filtering happens inside the database query.
func ownershipFilter(actor Actor) (uuid.UUID, bool) {
	if actor.ID == uuid.Nil {
		return uuid.Nil, false
	}
	if IsPrivilegedRole(actor.Role) {
		return uuid.Nil, true
	}
	return actor.ID, true
}
