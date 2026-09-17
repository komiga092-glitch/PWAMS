package services_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// ---------------------------------------------------------------------------
// ROLE ESCALATION PROTECTION TESTS
// ---------------------------------------------------------------------------

func TestStep3_AdminCannotCreateSuperAdmin(t *testing.T) {
	if services.CanAssignRole(models.RoleAdmin, models.RoleSuperAdmin) {
		t.Fatal("Admin must NOT be able to create a Super Admin account")
	}
}

func TestStep3_StaffCannotCreateSuperAdmin(t *testing.T) {
	if services.CanAssignRole(models.RoleStaff, models.RoleSuperAdmin) {
		t.Fatal("Staff must NOT be able to create a Super Admin account")
	}
}

func TestStep3_VolunteerCannotCreateSuperAdmin(t *testing.T) {
	if services.CanAssignRole(models.RoleVolunteer, models.RoleSuperAdmin) {
		t.Fatal("Volunteer must NOT be able to create a Super Admin account")
	}
}

func TestStep3_DonorCannotCreateSuperAdmin(t *testing.T) {
	if services.CanAssignRole(models.RoleDonor, models.RoleSuperAdmin) {
		t.Fatal("Donor must NOT be able to create a Super Admin account")
	}
}

func TestStep3_BeneficiaryCannotCreateSuperAdmin(t *testing.T) {
	if services.CanAssignRole(models.RoleBeneficiary, models.RoleSuperAdmin) {
		t.Fatal("Beneficiary must NOT be able to create a Super Admin account")
	}
}

func TestStep3_StudentCannotCreateSuperAdmin(t *testing.T) {
	if services.CanAssignRole(models.RoleStudent, models.RoleSuperAdmin) {
		t.Fatal("Student must NOT be able to create a Super Admin account")
	}
}

func TestStep3_PartnerCannotCreateSuperAdmin(t *testing.T) {
	if services.CanAssignRole(models.RolePartner, models.RoleSuperAdmin) {
		t.Fatal("Manager (Partner role) must NOT be able to create a Super Admin account")
	}
}

// ---------------------------------------------------------------------------
// ADMIN MANAGEMENT TESTS
// ---------------------------------------------------------------------------

func TestStep3_AdminCanCreateAdmin(t *testing.T) {
	if !services.CanAssignRole(models.RoleAdmin, models.RoleAdmin) {
		t.Fatal("Admin must be able to create another Admin account (multi-Admin)")
	}
}

func TestStep3_AdminCanCreateManager(t *testing.T) {
	if !services.CanAssignRole(models.RoleAdmin, models.RolePartner) {
		t.Fatal("Admin must be able to create a Manager (Partner role) account")
	}
}

func TestStep3_AdminCanCreateStaff(t *testing.T) {
	if !services.CanAssignRole(models.RoleAdmin, models.RoleStaff) {
		t.Fatal("Admin must be able to create Staff accounts")
	}
}

func TestStep3_AdminCanCreateVolunteer(t *testing.T) {
	if !services.CanAssignRole(models.RoleAdmin, models.RoleVolunteer) {
		t.Fatal("Admin must be able to create Volunteer accounts")
	}
}

func TestStep3_AdminCanCreateDonor(t *testing.T) {
	if !services.CanAssignRole(models.RoleAdmin, models.RoleDonor) {
		t.Fatal("Admin must be able to create Donor accounts")
	}
}

func TestStep3_AdminCanCreateBeneficiary(t *testing.T) {
	if !services.CanAssignRole(models.RoleAdmin, models.RoleBeneficiary) {
		t.Fatal("Admin must be able to create Beneficiary accounts")
	}
}

func TestStep3_AdminCanCreateStudent(t *testing.T) {
	if !services.CanAssignRole(models.RoleAdmin, models.RoleStudent) {
		t.Fatal("Admin must be able to create Student accounts")
	}
}

// ---------------------------------------------------------------------------
// SUPER ADMIN SECURITY TESTS
// ---------------------------------------------------------------------------

func TestStep3_SuperAdminCannotCreateSuperAdmin(t *testing.T) {
	if services.CanAssignRole(models.RoleSuperAdmin, models.RoleSuperAdmin) {
		t.Fatal("Super Admin must NOT be able to create another Super Admin account")
	}
}

func TestStep3_AdminCannotManageSuperAdmin(t *testing.T) {
	if services.CanManageAccountRole(models.RoleAdmin, models.RoleSuperAdmin) {
		t.Fatal("Admin must NOT be able to manage Super Admin accounts")
	}
}

func TestStep3_NonAdminRolesCannotManageSuperAdmin(t *testing.T) {
	for _, role := range []string{
		models.RolePartner,
		models.RoleStaff,
		models.RoleVolunteer,
		models.RoleDonor,
		models.RoleBeneficiary,
		models.RoleStudent,
	} {
		if services.CanManageAccountRole(role, models.RoleSuperAdmin) {
			t.Fatalf("%s must NOT be able to manage Super Admin accounts", role)
		}
	}
}

// ---------------------------------------------------------------------------
// LAST ACTIVE ADMIN PROTECTION
// ---------------------------------------------------------------------------

func TestStep3_LastActiveAdminProtected(t *testing.T) {
	db := step3TestDB(t)
	roleRepo := repository.NewRoleRepository(db)
	userRepo := repository.NewUserRepository(db)
	sessionRepo := repository.NewSessionRepository(db)
	svc := services.NewUserService(userRepo, roleRepo, sessionRepo)

	var adminRole models.Role
	if err := db.Where("name = ?", models.RoleAdmin).First(&adminRole).Error; err != nil {
		t.Fatalf("resolve Admin role: %v", err)
	}
	// Clean up existing Admins to start from a known state. Use a transaction
	// to ensure all related records are deleted atomically.
	err := db.Transaction(func(tx *gorm.DB) error {
		var existingAdmins []models.User
		tx.Unscoped().Where("role_id = ?", adminRole.ID).Find(&existingAdmins)
		for _, a := range existingAdmins {
			_ = tx.Where("user_id = ?", a.ID).Delete(&models.AuditLog{}).Error
			_ = tx.Where("entity_id = ?", a.ID).Delete(&models.AuditLog{}).Error
			_ = tx.Unscoped().Where("user_id = ?", a.ID).Delete(&models.PasswordResetToken{}).Error
			_ = tx.Unscoped().Where("user_id = ?", a.ID).Delete(&models.Session{}).Error
			_ = tx.Exec("DELETE FROM account_activation_tokens WHERE user_id = ?", a.ID).Error
			_ = tx.Exec("DELETE FROM notifications WHERE user_id = ?", a.ID).Error
			_ = tx.Exec("DELETE FROM messages WHERE sender_id = ? OR recipient_id = ?", a.ID, a.ID).Error
			_ = tx.Exec("DELETE FROM file_uploads WHERE user_id = ?", a.ID).Error
			_ = tx.Exec("DELETE FROM persons WHERE created_by_id = ?", a.ID).Error
			_ = tx.Exec("DELETE FROM students WHERE created_by_id = ?", a.ID).Error
			_ = tx.Exec("DELETE FROM deactivation_requests WHERE target_user_id = ? OR requester_id = ? OR responded_by_id = ?", a.ID, a.ID, a.ID).Error
			_ = tx.Exec("DELETE FROM admin_deletion_requests WHERE target_user_id = ? OR requester_id = ? OR responded_by_id = ?", a.ID, a.ID, a.ID).Error
		}
		_ = tx.Unscoped().Where("role_id = ?", adminRole.ID).Delete(&models.User{}).Error
		return nil
	})
	if err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}

	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	admin, err := svc.CreateUser(models.CreateUserRequest{
		Email:           "last-admin-" + suffix + "@pwams.local",
		FullName:        "Last Admin",
		Role:            models.RoleAdmin,
		Password:        "TestPassword123!",
		ConfirmPassword: "TestPassword123!",
	})
	if err != nil {
		t.Fatalf("Admin creation failed: %v", err)
	}

	err = svc.UpdateUserStatus(admin.ID.String(), models.UserStatusDisabled, models.RoleSuperAdmin)
	if err == nil {
		t.Fatal("deactivating the last active Admin must fail with ErrLastActiveAdmin")
	}
	if !strings.Contains(err.Error(), "at least one active Admin") {
		t.Fatalf("expected ErrLastActiveAdmin, got: %v", err)
	}
}
