package services

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
)

var (
	ErrPermissionNotFound = errors.New("permission not found")
	ErrRoleNotFound       = errors.New("role not found")
)

// PermissionService provides permission checking, default-permission
// resolution, and grant/revoke for user-specific overrides.
type PermissionService struct {
	permissionRepo *repository.PermissionRepository
	roleRepo       *repository.RoleRepository
	auditLogRepo   *repository.AuditLogRepository
}

func NewPermissionService(
	permissionRepo *repository.PermissionRepository,
	roleRepo *repository.RoleRepository,
	auditLogRepo *repository.AuditLogRepository,
) *PermissionService {
	return &PermissionService{
		permissionRepo: permissionRepo,
		roleRepo:       roleRepo,
		auditLogRepo:   auditLogRepo,
	}
}

// SeedDefaults creates every permission and applies role-based defaults.
// Safe to call on every startup (idempotent).
//
// Permission catalog: every permission in the canonical catalog is created
// if missing and its description/category refreshed. Never deletes.
//
// Role defaults: the built-in default matrix is applied ONLY to roles that
// have no permission assignments yet (fresh databases and newly introduced
// roles). Roles that already carry assignments — including sets customised
// at runtime through the permission designer — are left untouched so
// administrator changes survive restarts. The Super Admin role is the one
// exception: it is always repaired to hold the complete catalog, keeping
// the security invariant that the system owner can never be locked out of
// the protected permission namespaces.
func (s *PermissionService) SeedDefaults() error {
	perms := allPermissions()

	for _, p := range perms {
		existing, err := s.permissionRepo.FindByName(p.Name)
		if err == nil && existing != nil {
			if err := s.permissionRepo.Update(existing.ID, p.Description, p.Category); err != nil {
				return fmt.Errorf("failed to update permission %s: %w", p.Name, err)
			}
			continue
		}
		if err := s.permissionRepo.Create(&p); err != nil {
			return fmt.Errorf("failed to seed permission %s: %w", p.Name, err)
		}
	}

	defaults := defaultRolePermissions()

	for roleName, permNames := range defaults {
		role, err := s.roleRepo.FindByName(roleName)
		if err != nil {
			return fmt.Errorf("role %s not found during permission seed: %w", roleName, err)
		}

		// Leave curated role permission sets alone; only seed roles that
		// have never been configured. Super Admin is always repaired to the
		// full catalog (security invariant, see doc comment).
		if roleName != models.RoleSuperAdmin {
			count, err := s.permissionRepo.CountRolePermissions(role.ID)
			if err != nil {
				return fmt.Errorf("failed to inspect permissions for role %s: %w", roleName, err)
			}
			if count > 0 {
				continue
			}
		}

		permObjs, err := s.permissionRepo.FindByNames(permNames)
		if err != nil {
			return fmt.Errorf("failed to find permissions for role %s: %w", roleName, err)
		}

		permIDs := make([]uuid.UUID, 0, len(permObjs))
		for _, p := range permObjs {
			permIDs = append(permIDs, p.ID)
		}

		if err := s.permissionRepo.SetRolePermissions(role.ID, permIDs); err != nil {
			return fmt.Errorf("failed to set permissions for role %s: %w", roleName, err)
		}
	}
	// Self-service module safety net: roles that exist for their own data
	// (Donor, Beneficiary, Student) must never carry permissions for the
	// operational modules (donations, loans, repayments, care provided, aid
	// requests, students, donors, persons, users). The curated-matrix rule
	// above intentionally preserves already-configured roles, which means a
	// database configured before the donation module existed (or before the
	// permission catalog was introduced) could still carry donation.* grants
	// for these roles. Repair them to the canonical matrix on every seed so
	// the UI navigation and the backend gates agree (defense in depth; the
	// /donations role gate already rejects these roles independently).
	//
	// Operational roles are untouched: their sets stay curated by the
	// permission designer exactly as with the default matrix above.
	for roleName, permNames := range defaults {
		if !roleIsSelfServiceOnly(roleName) {
			continue
		}
		role, err := s.roleRepo.FindByName(roleName)
		if err != nil {
			return fmt.Errorf("role %s not found during permission seed: %w", roleName, err)
		}
		permObjs, err := s.permissionRepo.FindByNames(permNames)
		if err != nil {
			return fmt.Errorf("failed to find permissions for role %s: %w", roleName, err)
		}
		permIDs := make([]uuid.UUID, 0, len(permObjs))
		for _, p := range permObjs {
			permIDs = append(permIDs, p.ID)
		}
		if err := s.permissionRepo.SetRolePermissions(role.ID, permIDs); err != nil {
			return fmt.Errorf("failed to repair permissions for role %s: %w", roleName, err)
		}
	}

	// Admin-role repair: the Admin role must ALWAYS hold every admin.*
	// permission in the catalog (multi-Admin support — Admin account
	// management). Databases whose Admin role was configured before new
	// admin.* capabilities were introduced would otherwise never receive
	// them, because the default matrix above is only applied to roles with
	// zero permissions. The repair is add-only (existing grants — including
	// customised sets — are never revoked) and mirrors the Super Admin
	// full-catalog repair: an Admin must always be able to manage other
	// Admin accounts. This grants the ACCOUNT-MANAGEMENT namespace only;
	// it never grants permission_management.* or any other designer
	// capability, so the Admin role definition itself stays Super-Admin-only.
	adminRole, err := s.roleRepo.FindByName(models.RoleAdmin)
	if err != nil {
		return fmt.Errorf("role %s not found during permission seed: %w", models.RoleAdmin, err)
	}
	for _, p := range perms {
		if !strings.HasPrefix(strings.ToLower(p.Name), "admin.") {
			continue
		}
		existing, err := s.permissionRepo.FindByName(p.Name)
		if err != nil || existing == nil {
			continue
		}
		if err := s.permissionRepo.EnsureRolePermission(adminRole.ID, existing.ID); err != nil {
			return fmt.Errorf("failed to ensure admin.* permission %s for role %s: %w", p.Name, models.RoleAdmin, err)
		}
	}

	// Canonically-negated permission repair. Older seeds and historical
	// bugs could persist grants that the canonical default matrix never
	// contained for a role (e.g. donation.view on the self-service-only
	// Student role). Because the default matrix is only applied to roles
	// with zero assignments, such a stale grant survived every restart and
	// silently widened the role's effective permissions — including leaking
	// the donation module into navigation the role must never see.
	//
	// On every startup, remove from every non-SuperAdmin role any persisted
	// grant of a permission the canonical matrix says the role must NOT
	// hold. Grants the matrix does allow (including administrator-curated
	// extra grants beyond the defaults) are never touched, so this repair
	// is strictly subtractive, idempotent and deterministic.
	for roleName, permNames := range defaults {
		if roleName == models.RoleSuperAdmin {
			continue
		}
		role, err := s.roleRepo.FindByName(roleName)
		if err != nil {
			return fmt.Errorf("role %s not found during permission repair: %w", roleName, err)
		}
		mustHold := make(map[string]bool, len(permNames))
		for _, name := range permNames {
			mustHold[strings.ToLower(strings.TrimSpace(name))] = true
		}

		var held []models.Permission
		if err := s.permissionRepo.ListRolePermissions(role.ID, &held); err != nil {
			return fmt.Errorf("failed to load permissions for role %s: %w", roleName, err)
		}

		for _, p := range held {
			if mustHold[strings.ToLower(strings.TrimSpace(p.Name))] {
				continue
			}
			if err := s.permissionRepo.RemoveRolePermission(role.ID, p.ID); err != nil {
				return fmt.Errorf("failed to repair permission %s for role %s: %w", p.Name, roleName, err)
			}
		}
	}

	return nil
}

// GetUserPermissions returns the effective permission set for a user.
func (s *PermissionService) GetUserPermissions(user *models.User) ([]string, error) {
	return s.permissionRepo.GetUserPermissions(user)
}

// HasPermission checks whether a user has a given permission.
func (s *PermissionService) HasPermission(user *models.User, permission string) (bool, error) {
	perms, err := s.permissionRepo.GetUserPermissions(user)
	if err != nil {
		return false, err
	}

	target := strings.ToLower(strings.TrimSpace(permission))
	for _, p := range perms {
		if strings.ToLower(p) == target {
			return true, nil
		}
	}

	return false, nil
}

// HasAllPermissions checks that the user has every permission in the list.
func (s *PermissionService) HasAllPermissions(user *models.User, permissions ...string) (bool, error) {
	perms, err := s.permissionRepo.GetUserPermissions(user)
	if err != nil {
		return false, err
	}

	permSet := make(map[string]bool, len(perms))
	for _, p := range perms {
		permSet[strings.ToLower(p)] = true
	}

	for _, required := range permissions {
		if !permSet[strings.ToLower(strings.TrimSpace(required))] {
			return false, nil
		}
	}

	return true, nil
}

// GrantUserPermission grants or revokes a user-specific permission override.
func (s *PermissionService) GrantUserPermission(
	actorID, userID uuid.UUID,
	permissionName string,
	granted bool,
) error {
	permObj, err := s.permissionRepo.FindByName(permissionName)
	if err != nil {
		return ErrPermissionNotFound
	}

	if err := s.permissionRepo.GrantUserPermission(userID, permObj.ID, granted); err != nil {
		return fmt.Errorf("failed to grant/revoke user permission: %w", err)
	}

	if s.auditLogRepo != nil {
		action := "GRANT_PERMISSION"
		if !granted {
			action = "REVOKE_PERMISSION"
		}
		permID := permObj.ID
		_ = s.auditLogRepo.Create(&models.AuditLog{
			UserID:   &actorID,
			Action:   action,
			Entity:   "permissions",
			EntityID: &permID,
			Details:  fmt.Sprintf("%s permission %s for user %s", action, permObj.Name, userID),
		})
	}

	return nil
}

// SetRolePermissions assigns permissions to a role by permission name strings.
func (s *PermissionService) SetRolePermissions(roleName string, permNames []string) error {
	role, err := s.roleRepo.FindByName(roleName)
	if err != nil {
		return ErrRoleNotFound
	}

	permObjs, err := s.permissionRepo.FindByNames(permNames)
	if err != nil {
		return fmt.Errorf("failed to find permissions: %w", err)
	}

	permIDs := make([]uuid.UUID, 0, len(permObjs))
	for _, p := range permObjs {
		permIDs = append(permIDs, p.ID)
	}

	return s.permissionRepo.SetRolePermissions(role.ID, permIDs)
}

// GetRolePermissions returns the permission names assigned to a role.
func (s *PermissionService) GetRolePermissions(roleName string) ([]string, error) {
	return s.permissionRepo.GetRolePermissionsByRoleName(roleName)
}

// ListRoles returns all roles for the permission-management UI.
func (s *PermissionService) ListRoles() ([]models.Role, error) {
	return s.roleRepo.List()
}

// ErrPermissionDenied indicates an actor attempted an unauthorized
// permission-management operation.
var ErrPermissionDenied = errors.New("you are not authorised to perform this action")

// ErrProtectedPermission indicates an attempt to assign one of the protected
// permissions (admin.*, super_admin.*, system.*, protected_rbac.*) to
// anything other than the Super Admin role.
var ErrProtectedPermission = errors.New(
	"protected permissions can only be held by the Super Admin role",
)

// CanManageRolePermissions returns nil when the actor may change the given
// role's permission set. As a privilege-escalation guard, only Super Admins
// may modify the Super Admin or Admin roles; other actors may only manage
// roles for which they could assign members.
//
// The guard is intentionally a pure function of the actor's role and the
// target role name: every decision is taken before any repository access so
// the boundary cannot be bypassed and can be unit-tested without a database.
// Role existence is validated by SetRolePermissionsByActor when the change
// is actually applied.
func (s *PermissionService) CanManageRolePermissions(actor *models.User, roleName string) error {
	if actor == nil {
		return ErrPermissionDenied
	}

	if actor.Role.Name == models.RoleSuperAdmin {
		return nil
	}

	// Only Super Admins manage the Super Admin and Admin roles.
	if roleName == models.RoleSuperAdmin || roleName == models.RoleAdmin {
		return ErrPermissionDenied
	}

	// The actor must at minimum be able to assign members of this role.
	if !canCreateRoleForActor(actor, roleName) {
		return ErrPermissionDenied
	}

	return nil
}

func canCreateRoleForActor(actor *models.User, roleName string) bool {
	if actor == nil {
		return false
	}

	return CanAssignRole(actor.Role.Name, roleName)
}

// SetRolePermissionsByActor replaces a role's permission set after enforcing
// the actor's management boundaries. Every change is audited.
//
// Protected permissions (admin.*, super_admin.*, system.*,
// protected_rbac.*) may ONLY ever be granted to the Super Admin role: any
// request that contains them for another role is rejected outright, even
// when the actor is a Super Admin. This guarantees the permission-management
// API can never be used to hand the system-administration namespace to a
// lower-privileged role (direct privilege-escalation guard).
func (s *PermissionService) SetRolePermissionsByActor(
	actor *models.User,
	roleName string,
	permNames []string,
) error {
	if err := s.CanManageRolePermissions(actor, roleName); err != nil {
		return err
	}

	if roleName != models.RoleSuperAdmin {
		// Role-aware protected-permission filter: the Admin role may hold
		// admin.* (it manages other Admin accounts); no other role may hold
		// any protected permission. Any request that contains a permission the
		// target role may not hold is rejected outright, even when the actor
		// is a Super Admin. This guarantees the permission-management API can
		// never be used to hand the system-administration namespace to a
		// lower-privileged role (direct privilege-escalation guard).
		_, blocked := filterPermissionsForRole(roleName, permNames)
		if len(blocked) > 0 {
			return fmt.Errorf(
				"%w: %s",
				ErrProtectedPermission,
				strings.Join(blocked, ", "),
			)
		}
	}

	role, err := s.roleRepo.FindByName(roleName)
	if err != nil {
		return ErrRoleNotFound
	}

	permObjs, err := s.permissionRepo.FindByNames(permNames)
	if err != nil {
		return fmt.Errorf("failed to find permissions: %w", err)
	}

	permIDs := make([]uuid.UUID, 0, len(permObjs))
	for _, p := range permObjs {
		permIDs = append(permIDs, p.ID)
	}

	if err := s.permissionRepo.SetRolePermissions(role.ID, permIDs); err != nil {
		return err
	}

	if s.auditLogRepo != nil {
		roleID := role.ID
		_ = s.auditLogRepo.Create(&models.AuditLog{
			UserID:   &actor.ID,
			Action:   "ROLE_PERMISSIONS_UPDATED",
			Entity:   "roles",
			EntityID: &roleID,
			Details:  fmt.Sprintf("Updated permission set for role %s (actor %s)", roleName, actor.ID),
		})
	}

	return nil
}

// CanManageUserPermissions returns nil when the actor may apply permission
// overrides to the target user. Actors may only manage users whose role they
// could assign, and never Super Admin or Admin targets unless the actor is a
// Super Admin.
func (s *PermissionService) CanManageUserPermissions(actor, target *models.User) error {
	if actor == nil || target == nil {
		return ErrPermissionDenied
	}

	if actor.Role.Name == models.RoleSuperAdmin {
		return nil
	}

	if target.Role.Name == models.RoleSuperAdmin || target.Role.Name == models.RoleAdmin {
		return ErrPermissionDenied
	}

	if !canCreateRoleForActor(actor, target.Role.Name) {
		return ErrPermissionDenied
	}

	return nil
}

// UserPermissionOverride is a single grant/revoke request for a user.
type UserPermissionOverride struct {
	Name    string `json:"name"`
	Granted bool   `json:"granted"`
}

// ApplyUserPermissionOverrides applies user-specific permission overrides.
// The actor boundary is enforced through CanManageUserPermissions, and every
// change is individually audited.
func (s *PermissionService) ApplyUserPermissionOverrides(
	actor, target *models.User,
	overrides []UserPermissionOverride,
) error {
	if target == nil {
		return repository.ErrUserNotFound
	}

	if err := s.CanManageUserPermissions(actor, target); err != nil {
		return err
	}

	for _, override := range overrides {
		name := strings.ToLower(strings.TrimSpace(override.Name))
		if name == "" {
			continue
		}

		// Protected permissions (admin.*, super_admin.*, system.*,
		// protected_rbac.*) must never leave the Super Admin role. Grants
		// through user-specific overrides are rejected unconditionally —
		// including by Super Admins, since the role permission matrix is
		// the only sanctioned way to hold them. Revocations remain
		// allowed so accidental grants can always be undone.
		if override.Granted && IsProtectedPermission(name) {
			return fmt.Errorf("%w: %s", ErrProtectedPermission, name)
		}

		perm, err := s.permissionRepo.FindByName(name)
		if err != nil {
			return ErrPermissionNotFound
		}

		if err := s.permissionRepo.GrantUserPermission(target.ID, perm.ID, override.Granted); err != nil {
			return fmt.Errorf("failed to apply permission override: %w", err)
		}

		s.recordPermissionAudit(actor.ID, target.ID, perm.Name, override.Granted)
	}

	return nil
}

// GetUserPermissionOverrides returns the explicit overrides for a user.
func (s *PermissionService) GetUserPermissionOverrides(userID uuid.UUID) ([]models.UserPermission, error) {
	return s.permissionRepo.ListUserPermissionOverrides(userID)
}

func (s *PermissionService) recordPermissionAudit(actorID, targetID uuid.UUID, permissionName string, granted bool) {
	if s.auditLogRepo == nil {
		return
	}
	action := "GRANT_PERMISSION"
	if !granted {
		action = "REVOKE_PERMISSION"
	}
	permID := targetID // entity reference is the target user
	_ = s.auditLogRepo.Create(&models.AuditLog{
		UserID:   &actorID,
		Action:   action,
		Entity:   "user_permissions",
		EntityID: &permID,
		Details:  fmt.Sprintf("%s %s for user %s", action, permissionName, targetID),
	})
}
