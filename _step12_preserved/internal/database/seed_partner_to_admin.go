package database

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/config"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/services"
)

var (
	// ErrPartnerToAdminNotConfigured reports a call without the operator
	// supplied identifier; callers should check for an empty identifier first.
	ErrPartnerToAdminNotConfigured = errors.New("manager-to-admin conversion requires an identifier")
	// ErrPartnerToAdminNotFound reports that no user matches the configured
	// unique identifier (email) while holding the Partner role (Manager).
	ErrPartnerToAdminNotFound = errors.New("manager-to-admin conversion: no Manager (Partner role) account matches the configured identifier")
	// ErrPartnerToAdminNotPartner reports that the identifier resolved to a
	// user that exists but does not hold the Partner role (Manager). The conversion
	// refuses to touch accounts of any other role.
	ErrPartnerToAdminNotPartner = errors.New("manager-to-admin conversion: the identified account does not hold the Partner role (Manager)")
)

// PromoteConfiguredPartnerToAdmin converts exactly ONE existing Manager
// (Partner role) account to Admin, transactionally, preserving every attribute of the
// account except its role. The intended account is identified by the
// operator-supplied unique identifier in cfg.PartnerToAdminEmail (an email).
//
// Guarantees:
//
//   - The identifier is never guessed: when PARTNER_TO_ADMIN_EMAIL is empty
//     the function is a no-op; when it is set, exactly the account whose
//     email matches is considered. Every other Manager (Partner role) account is untouched,
//     and if multiple Manager accounts exist only the matched one converts.
//   - User ID, username, email, password hash, person/profile data, tenant
//     association, related records and created_at are preserved — only
//     RoleID is updated to the existing Admin role record.
//   - The role flip runs in a single transaction together with the audit
//     record, so the conversion is atomic and audited
//     (PARTNER_TO_ADMIN_ROLE_CHANGE with old value = Partner and new value =
//     Admin).
//   - Idempotent: running the server repeatedly (or re-running the seed) can
//     never create a duplicate user and re-driving a completed conversion is
//     a no-op.
//
// actorUserID is the seed actor recorded in the audit log (the operator /
// system owner). Pass "" and the audit row records a nil actor.
func PromoteConfiguredPartnerToAdmin(
	db *gorm.DB,
	cfg *config.Config,
	auditLogService *services.AuditLogService,
	actorUserID string,
) (*models.User, error) {
	identifier := strings.ToLower(strings.TrimSpace(cfg.PartnerToAdminEmail))
	if identifier == "" {
		return nil, nil
	}

	if db == nil {
		return nil, fmt.Errorf("manager-to-admin conversion: nil database")
	}

	var (
		adminRole   models.Role
		partnerRole models.Role
	)

	if err := db.Where("name = ?", models.RoleAdmin).First(&adminRole).Error; err != nil {
		return nil, fmt.Errorf("partner-to-admin conversion: Admin role not found: %w", err)
	}
	if err := db.Where("name = ?", models.RolePartner).First(&partnerRole).Error; err != nil {
		return nil, fmt.Errorf("manager-to-admin conversion: Partner role (Manager) not found: %w", err)
	}

	var converted *models.User

	err := db.Transaction(func(tx *gorm.DB) error {
		// Load the candidate by the unique email. The de-duplicating unique
		// index on email means at most one row can ever match. Deliberately
		// NO Preload here: role membership is decided from the authoritative
		// role_id foreign keys below, and a populated Role association on the
		// struct passed to a later Update makes GORM re-assert the stale
		// foreign key, silently reverting the role change (observed against
		// PostgreSQL: UPDATE reports rows=1 while the row keeps the old
		// role_id).
		var target models.User
		err := tx.
			Where("LOWER(email) = ?", identifier).
			First(&target).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrPartnerToAdminNotFound
			}
			return fmt.Errorf("manager-to-admin conversion: lookup failed: %w", err)
		}

		// Idempotency: the account is already an Admin (either converted by a
		// previous run or already created as Admin) — nothing to do.
		if target.RoleID == adminRole.ID {
			converted = &target
			return nil
		}

		// Safety: never convert an account that does not currently hold the
		// Partner role (Manager). Nobody — including the operator — may silently
		// re-role a Super Admin, Staff, Donor etc. through the seed.
		if target.RoleID != partnerRole.ID {
			return ErrPartnerToAdminNotPartner
		}

		// Update through a clean model struct (never through the loaded one):
		// only the role changes; every other column is untouched.
		if err := tx.Model(&models.User{}).
			Where("id = ?", target.ID).
			Update("role_id", adminRole.ID).Error; err != nil {
			return fmt.Errorf("manager-to-admin conversion: role update failed: %w", err)
		}

		// Read the row back INSIDE the transaction so a silently reverted
		// update can never be reported as success.
		var verified models.User
		if err := tx.
			Select("id", "role_id", "username", "email", "password_hash", "status", "created_at").
			Where("id = ?", target.ID).
			First(&verified).Error; err != nil {
			return fmt.Errorf("manager-to-admin conversion: verification read failed: %w", err)
		}
		if verified.RoleID != adminRole.ID {
			return fmt.Errorf(
				"manager-to-admin conversion: role change did not persist (role_id still %s)",
				verified.RoleID,
			)
		}

		target.RoleID = adminRole.ID
		target.Role = adminRole
		converted = &target

		// Audit atomically with the conversion so the record can never exist
		// without the state change it describes.
		if auditLogService != nil {
			if err := auditLogService.CreateTx(
				tx,
				actorUserID,
				"PARTNER_TO_ADMIN_ROLE_CHANGE",
				"users",
				target.ID.String(),
				fmt.Sprintf("Manager (Partner role) account %s promoted to Admin (role change only)", target.ID),
				models.RolePartner,
				models.RoleAdmin); err != nil {
				return fmt.Errorf("manager-to-admin conversion: audit failed: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return converted, nil
}
