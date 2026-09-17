package services_test

import (
	"testing"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
	"github.com/shopspring/decimal"
)

// TestDonationObjectLevelAuthorization verifies the service-layer IDOR guard
// on Donation endpoints: the creating principal and privileged roles may
// read/update/delete/cancel, while unprivileged cross-users (Donor,
// Beneficiary, Student) are refused with ErrRecordAccessDenied and cause
// no mutation.
func TestDonationObjectLevelAuthorization(t *testing.T) {
	SkipUnlessForceIntegration(t)
	db := AcquireTestDB(t)
	fx := newFixture(db)
	defer fx.Cleanup()

	roles := loadSeedRoles(t, db)
	donationRepo := repository.NewDonationRepository(db)
	donorRepo := repository.NewDonorRepository(db)
	personRepo := repository.NewPersonRepository(db)
	donationSvc := services.NewDonationService(donationRepo, donorRepo, personRepo)

	owner := makeUser(t, db, fx, roles.StaffID, models.RoleStaff, "owner")
	admin := makeUser(t, db, fx, roles.AdminID, models.RoleAdmin, "admin")
	superAdmin := makeUser(t, db, fx, roles.SuperAdminID, models.RoleSuperAdmin, "superadmin")
	donorUser := makeUser(t, db, fx, roles.DonorID, models.RoleDonor, "intruder")
	beneficiary := makeUser(t, db, fx, roles.BeneficiaryID, models.RoleBeneficiary, "intruder")
	studentUser := makeUser(t, db, fx, roles.StudentID, models.RoleStudent, "intruder")

	donor := makeDonorActive(t, db, fx, owner.ID, "owner")
	donation := makeDonation(t, db, fx, owner.ID, donor.ID)

	// Authorized owner can read.
	if _, err := donationSvc.GetDonationByID(donation.ID.String(), services.Actor{ID: owner.ID, Role: models.RoleStaff}); err != nil {
		t.Fatalf("owner read: unexpected error: %v", err)
	}

	// Privileged Admin can read.
	if _, err := donationSvc.GetDonationByID(donation.ID.String(), services.Actor{ID: admin.ID, Role: models.RoleAdmin}); err != nil {
		t.Fatalf("admin read: unexpected error: %v", err)
	}

	// Privileged Super Admin can read.
	if _, err := donationSvc.GetDonationByID(donation.ID.String(), services.Actor{ID: superAdmin.ID, Role: models.RoleSuperAdmin}); err != nil {
		t.Fatalf("super admin read: unexpected error: %v", err)
	}

	// Non-owner Donor is denied.
	if _, err := donationSvc.GetDonationByID(donation.ID.String(), services.Actor{ID: donorUser.ID, Role: models.RoleDonor}); err != services.ErrRecordAccessDenied {
		t.Fatalf("non-owner Donor read: expected ErrRecordAccessDenied, got: %v", err)
	}

	// Non-owner Beneficiary is denied.
	if _, err := donationSvc.GetDonationByID(donation.ID.String(), services.Actor{ID: beneficiary.ID, Role: models.RoleBeneficiary}); err != services.ErrRecordAccessDenied {
		t.Fatalf("non-owner Beneficiary read: expected ErrRecordAccessDenied, got: %v", err)
	}

	// Non-owner Student is denied.
	if _, err := donationSvc.GetDonationByID(donation.ID.String(), services.Actor{ID: studentUser.ID, Role: models.RoleStudent}); err != services.ErrRecordAccessDenied {
		t.Fatalf("non-owner Student read: expected ErrRecordAccessDenied, got: %v", err)
	}

	// Nil/unknown actor is denied.
	if _, err := donationSvc.GetDonationByID(donation.ID.String(), services.Actor{}); err != services.ErrRecordAccessDenied {
		t.Fatalf("nil actor read: expected ErrRecordAccessDenied, got: %v", err)
	}

	// Invalid record ID.
	if _, err := donationSvc.GetDonationByID("not-a-uuid", services.Actor{ID: owner.ID, Role: models.RoleStaff}); err != services.ErrInvalidDonationID {
		t.Fatalf("invalid id read: expected ErrInvalidDonationID, got: %v", err)
	}

	// Cross-user cannot update; state must not mutate.
	origAmount := donation.Amount
	if _, err := donationSvc.UpdateDonation(donation.ID.String(), models.UpdateDonationRequest{
		DonationType: models.DonationTypeCash,
		Amount:       decimal.NewFromInt(99999),
		Quantity:     decimal.Zero,
		Status:       models.DonationStatusPending,
	}, services.Actor{ID: donorUser.ID, Role: models.RoleDonor}); err != services.ErrRecordAccessDenied {
		t.Fatalf("cross-user update: expected ErrRecordAccessDenied, got: %v", err)
	}

	reloaded, err := donationRepo.FindByID(donation.ID.String())
	if err != nil {
		t.Fatalf("reload donation: %v", err)
	}
	if !reloaded.Amount.Equal(origAmount) {
		t.Fatalf("cross-user update mutated amount: got %v want %v", reloaded.Amount, origAmount)
	}

	// Cross-user cannot cancel (UpdateStatus).
	if err := donationSvc.UpdateDonationStatus(donation.ID.String(), models.DonationStatusCancelled, services.Actor{ID: beneficiary.ID, Role: models.RoleBeneficiary}); err != services.ErrRecordAccessDenied {
		t.Fatalf("cross-user cancel: expected ErrRecordAccessDenied, got: %v", err)
	}

	reloaded2, err := donationRepo.FindByID(donation.ID.String())
	if err != nil {
		t.Fatalf("reload donation after cancel attempt: %v", err)
	}
	if reloaded2.Status == models.DonationStatusCancelled {
		t.Fatalf("cross-user cancel mutated status")
	}

	// Cross-user cannot delete.
	if err := donationSvc.DeleteDonation(donation.ID.String(), services.Actor{ID: studentUser.ID, Role: models.RoleStudent}); err != services.ErrRecordAccessDenied {
		t.Fatalf("cross-user delete: expected ErrRecordAccessDenied, got: %v", err)
	}

	if _, err := donationRepo.FindByID(donation.ID.String()); err != nil {
		t.Fatalf("record missing after denied delete: %v", err)
	}

	// Owner can update.
	updated, err := donationSvc.UpdateDonation(donation.ID.String(), models.UpdateDonationRequest{
		DonationType: models.DonationTypeCash,
		Amount:       decimal.NewFromInt(2000),
		Quantity:     decimal.Zero,
		Status:       models.DonationStatusPending,
	}, services.Actor{ID: owner.ID, Role: models.RoleStaff})
	if err != nil {
		t.Fatalf("owner update: unexpected error: %v", err)
	}
	if !updated.Amount.Equal(decimal.NewFromInt(2000)) {
		t.Fatalf("owner update: expected amount 2000, got %v", updated.Amount)
	}

	// Owner can cancel.
	if err := donationSvc.UpdateDonationStatus(donation.ID.String(), models.DonationStatusCancelled, services.Actor{ID: owner.ID, Role: models.RoleStaff}); err != nil {
		t.Fatalf("owner cancel: unexpected error: %v", err)
	}

	// Confirmed donation cannot delete. Confirm it first.
	if err := donationSvc.UpdateDonationStatus(donation.ID.String(), models.DonationStatusConfirmed, services.Actor{ID: owner.ID, Role: models.RoleStaff}); err != nil {
		t.Fatalf("owner confirm: unexpected error: %v", err)
	}

	// Deletion of confirmed donation must be refused.
	if err := donationSvc.DeleteDonation(donation.ID.String(), services.Actor{ID: owner.ID, Role: models.RoleStaff}); err != services.ErrConfirmedDonationCannotDelete {
		t.Fatalf("confirmed delete: expected ErrConfirmedDonationCannotDelete, got: %v", err)
	}

	// Record must still exist.
	if _, err := donationRepo.FindByID(donation.ID.String()); err != nil {
		t.Fatalf("record missing after denied confirmed delete: %v", err)
	}
}
