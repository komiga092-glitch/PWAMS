package repository

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/models"
)

var ErrPermissionNotFound = errors.New("permission not found")

type PermissionRepository struct {
	db *gorm.DB
}

func NewPermissionRepository(db *gorm.DB) *PermissionRepository {
	return &PermissionRepository{db: db}
}

// FindByName returns a permission by its unique name (case-insensitive).
func (r *PermissionRepository) FindByName(name string) (*models.Permission, error) {
	var permission models.Permission

	err := r.db.
		Where("LOWER(name) = LOWER(?)", strings.TrimSpace(name)).
		First(&permission).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrPermissionNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("failed to find permission: %w", err)
	}

	return &permission, nil
}

// FindByID returns a permission by its UUID.
func (r *PermissionRepository) FindByID(id uuid.UUID) (*models.Permission, error) {
	var permission models.Permission

	err := r.db.
		Where("id = ?", id).
		First(&permission).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrPermissionNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("failed to find permission: %w", err)
	}

	return &permission, nil
}

// Create inserts a new permission.
func (r *PermissionRepository) Create(permission *models.Permission) error {
	return r.db.Create(permission).Error
}

// Update modifies the description and category of a permission.
func (r *PermissionRepository) Update(id uuid.UUID, description, category string) error {
	return r.db.
		Model(&models.Permission{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"description": description,
			"category":    category,
			"updated_at":  time.Now().UTC(),
		}).Error
}

// List returns all permissions ordered by category then name.
func (r *PermissionRepository) List() ([]models.Permission, error) {
	var permissions []models.Permission

	if err := r.db.Order("category, name").Find(&permissions).Error; err != nil {
		return nil, fmt.Errorf("failed to list permissions: %w", err)
	}

	return permissions, nil
}

// FindByNames returns permissions matching the given names (case-insensitive).
func (r *PermissionRepository) FindByNames(names []string) ([]models.Permission, error) {
	if len(names) == 0 {
		return []models.Permission{}, nil
	}

	lowers := make([]string, len(names))
	for i, n := range names {
		lowers[i] = strings.ToLower(strings.TrimSpace(n))
	}

	var permissions []models.Permission
	if err := r.db.
		Where("LOWER(name) IN (?)", lowers).
		Find(&permissions).Error; err != nil {
		return nil, fmt.Errorf("failed to find permissions: %w", err)
	}

	return permissions, nil
}

// GetRolePermissions returns all permission names granted to a role.
func (r *PermissionRepository) GetRolePermissions(roleID uuid.UUID) ([]string, error) {
	var permissions []models.Permission

	err := r.db.
		Model(&models.Permission{}).
		Joins("JOIN role_permissions ON role_permissions.permission_id = permissions.id").
		Where("role_permissions.role_id = ?", roleID).
		Find(&permissions).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get role permissions: %w", err)
	}

	return toStringSlice(permissions), nil
}

// GetRolePermissionsByRoleName returns all permission names granted to a role
// identified by its name (case-insensitive).
func (r *PermissionRepository) GetRolePermissionsByRoleName(roleName string) ([]string, error) {
	var permissions []models.Permission

	err := r.db.
		Model(&models.Permission{}).
		Joins("JOIN roles ON roles.id = role_permissions.role_id").
		Joins("JOIN role_permissions ON role_permissions.permission_id = permissions.id").
		Where("LOWER(roles.name) = LOWER(?)", strings.TrimSpace(roleName)).
		Find(&permissions).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get role permissions: %w", err)
	}

	return toStringSlice(permissions), nil
}

// GetUserPermissions returns all effective permission names for a user,
// combining role-based permissions with any user-specific grants/revocations.
func (r *PermissionRepository) GetUserPermissions(user *models.User) ([]string, error) {
	var permissions []models.Permission

	dbQuery := r.db.
		Model(&models.Permission{}).
		Joins("JOIN role_permissions ON role_permissions.permission_id = permissions.id").
		Where("role_permissions.role_id = ?", user.RoleID).
		Joins("LEFT JOIN user_permissions ON user_permissions.permission_id = permissions.id AND user_permissions.user_id = ?", user.ID).
		Where("user_permissions.granted IS NULL OR user_permissions.granted = ?", true).
		Group("permissions.id")

	if err := dbQuery.Find(&permissions).Error; err != nil {
		return nil, fmt.Errorf("failed to get user permissions: %w", err)
	}

	return toStringSlice(permissions), nil
}

func toStringSlice(permissions []models.Permission) []string {
	names := make([]string, 0, len(permissions))
	for _, p := range permissions {
		names = append(names, p.Name)
	}
	return names
}

// CountRolePermissions returns how many permissions are currently assigned
// to a role. SeedDefaults uses this to apply the built-in default matrix
// only to roles that have never been configured (fresh databases), so
// runtime edits made through the permission designer survive restarts.
func (r *PermissionRepository) CountRolePermissions(roleID uuid.UUID) (int64, error) {
	var count int64
	if err := r.db.
		Model(&models.RolePermission{}).
		Where("role_id = ?", roleID).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to count role permissions: %w", err)
	}
	return count, nil
}

// SetRolePermissions replaces all permissions for a role in a single
// transaction.
func (r *PermissionRepository) SetRolePermissions(
	roleID uuid.UUID,
	permissionIDs []uuid.UUID,
) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.
			Where("role_id = ?", roleID).
			Delete(&models.RolePermission{}).Error; err != nil {
			return fmt.Errorf("failed to clear existing role permissions: %w", err)
		}

		if len(permissionIDs) == 0 {
			return nil
		}

		rolePerms := make([]models.RolePermission, 0, len(permissionIDs))
		for _, pid := range permissionIDs {
			rolePerms = append(rolePerms, models.RolePermission{
				RoleID:       roleID,
				PermissionID: pid,
				CreatedAt:    time.Now().UTC(),
			})
		}

		return tx.Create(&rolePerms).Error
	})
}

// EnsureRolePermission grants a single permission to a role if it is not
// already granted (add-only; existing grants and revocations are untouched).
// SeedDefaults uses it to repair the Admin role's admin.* namespace on every
// startup without clobbering curated role permission sets the way
// SetRolePermissions would.
func (r *PermissionRepository) EnsureRolePermission(roleID, permissionID uuid.UUID) error {
	var count int64
	if err := r.db.Model(&models.RolePermission{}).
		Where("role_id = ? AND permission_id = ?", roleID, permissionID).
		Count(&count).Error; err != nil {
		return fmt.Errorf("failed to inspect role permission: %w", err)
	}
	if count > 0 {
		return nil
	}
	if err := r.db.Create(&models.RolePermission{
		RoleID:       roleID,
		PermissionID: permissionID,
	}).Error; err != nil {
		return fmt.Errorf("failed to grant role permission: %w", err)
	}
	return nil
}

// GrantUserPermission grants or revokes a permission for a specific user.
func (r *PermissionRepository) GrantUserPermission(
	userID, permissionID uuid.UUID,
	granted bool,
) error {
	var existing models.UserPermission
	err := r.db.
		Where("user_id = ? AND permission_id = ?", userID, permissionID).
		First(&existing).Error

	if err == nil {
		return r.db.Model(&existing).
			Update("granted", granted).Error
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("failed to check user permission: %w", err)
	}

	return r.db.Create(&models.UserPermission{
		UserID:       userID,
		PermissionID: permissionID,
		Granted:      granted,
	}).Error
}

// RevokeUserPermission removes a user-specific permission override so the
// user falls back to their role's default.
func (r *PermissionRepository) RevokeUserPermission(
	userID, permissionID uuid.UUID,
) error {
	return r.db.
		Where("user_id = ? AND permission_id = ?", userID, permissionID).
		Delete(&models.UserPermission{}).Error
}

// ListUserPermissionOverrides returns every user-specific permission override
// for a user, with the resolved permission name and granted state attached.
func (r *PermissionRepository) ListUserPermissionOverrides(
	userID uuid.UUID,
) ([]models.UserPermission, error) {
	var overrides []models.UserPermission

	err := r.db.
		Where("user_id = ?", userID).
		Order("permission_id").
		Find(&overrides).Error

	if err != nil {
		return nil, fmt.Errorf("failed to list user permission overrides: %w", err)
	}

	return overrides, nil
}
