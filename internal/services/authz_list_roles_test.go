package services_test

import (
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// TestListAuthorization_PrivilegedRolesRetainAccess verifies that all
// privileged roles (Super Admin, Admin, Manager/Partner, Staff,
// Volunteer) retain full cross-record access to list results: they can
// find a record they did not create. Search is scoped by ownership too,
// so a non-privileged actor with no owned records can never find it.
func TestListAuthorization_PrivilegedRolesRetainAccess(t *testing.T) {
	SkipUnlessForceIntegration(t)
	db := AcquireTestDB(t)
	fx := newFixture(db)
	defer fx.Cleanup()

	roles := loadSeedRoles(t, db)
	personRepo := repository.NewPersonRepository(db)
	personSvc := services.NewPersonService(personRepo)

	// A non-privileged Donor creates exactly one person.
	owner := makeUser(t, db, fx, roles.DonorID, models.RoleDonor, "owner")
	person := makePerson(t, db, fx, owner.ID, "privtgt")
	// Search by the unique NIC so results are exact regardless of DB history.
	search := person.NICPassport

	// Privileged users.
	admin := makeUser(t, db, fx, roles.AdminID, models.RoleAdmin, "admin")
	superAdmin := makeUser(t, db, fx, roles.SuperAdminID, models.RoleSuperAdmin, "superadmin")
	staff := makeUser(t, db, fx, roles.StaffID, models.RoleStaff, "staff")
	volunteerRoleID := roleIDByName(t, db, models.RoleVolunteer)
	volunteer := makeUser(t, db, fx, volunteerRoleID, models.RoleVolunteer, "volunteer")

	// Partner/Manager — reuse existing if present (single-manager constraint).
	var manager models.User
	if partnerID := existingPartnerID(db); partnerID != uuid.Nil {
		if err := db.Preload("Role").First(&manager, "id = ?", partnerID).Error; err != nil {
			t.Fatalf("failed to load existing Partner: %v", err)
		}
	} else {
		manager = makeUser(t, db, fx, roles.PartnerID, models.RoleManager, "manager")
	}

	privilegedActors := []services.Actor{
		{ID: admin.ID, Role: models.RoleAdmin},
		{ID: superAdmin.ID, Role: models.RoleSuperAdmin},
		{ID: staff.ID, Role: models.RoleStaff},
		{ID: volunteer.ID, Role: models.RoleVolunteer},
		{ID: manager.ID, Role: models.RoleManager},
	}

	// Each privileged role must find the record it did not create.
	for _, actor := range privilegedActors {
		persons, total, _, _, err := personSvc.ListPersons(models.PersonListQuery{Search: search, Page: 1, PageSize: 50}, actor)
		if err != nil {
			t.Fatalf("privileged %s search: unexpected error: %v", actor.Role, err)
		}
		if total != 1 || len(persons) != 1 {
			t.Fatalf("privileged %s search: expected 1 result, got %d (privilege regression)", actor.Role, len(persons))
		}
	}
}

// roleIDByName returns the role ID for the given role name.
func roleIDByName(t *testing.T, db *gorm.DB, roleName string) uuid.UUID {
	t.Helper()
	var role models.Role
	if db.Where("LOWER(name) = LOWER(?)", roleName).Take(&role).Error != nil {
		return uuid.UUID{}
	}
	return role.ID
}
