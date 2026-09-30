package models

import "strings"

// Partner → Manager compatibility layer.
//
// The role formerly displayed as "Partner" is, as of Phase 2, user-facing
// "Manager". "Partner" survives only as a legacy input/display alias so that
// existing database records and old API payloads keep working. No duplicate
// Manager role constant is created: RoleManager (above) stays canonical.

const LegacyRolePartner = "Partner"

// NormalizeRoleInput maps legacy role names to their canonical names and
// trims surrounding whitespace. Legacy "Partner" input normalizes to Manager.
func NormalizeRoleInput(role string) string {
	trimmed := strings.TrimSpace(role)
	if strings.EqualFold(trimmed, LegacyRolePartner) {
		return RoleManager
	}
	return trimmed
}

// IsManagerRole reports whether a stored or supplied role is the NGO
// Manager role, accepting the legacy "Partner" alias.
func IsManagerRole(role string) bool {
	return NormalizeRoleInput(role) == RoleManager
}

// RoleDisplayName returns the user-facing label for a role. Legacy Partner
// records display as Manager.
func RoleDisplayName(role string) string {
	if IsManagerRole(role) {
		return RoleManager
	}
	return role
}