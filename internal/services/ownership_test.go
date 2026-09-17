package services_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// Regression tests for the STEP 15.5 IDOR fix: object-level ownership
// enforcement at the service layer for Aid Requests, Loans, Loan
// Repayments, and Care Provided.

func ownerActor() services.Actor {
	return services.Actor{ID: uuid.MustParse("00000000-0000-0000-0000-00000000aa01"), Role: models.RoleBeneficiary}
}

func otherBeneficiaryActor() services.Actor {
	return services.Actor{ID: uuid.MustParse("00000000-0000-0000-0000-00000000aa02"), Role: models.RoleBeneficiary}
}

func otherDonorActor() services.Actor {
	return services.Actor{ID: uuid.MustParse("00000000-0000-0000-0000-00000000aa03"), Role: models.RoleDonor}
}

func TestCanAccessRecord_OwnerAllowed(t *testing.T) {
	owner := ownerActor()

	if !services.CanAccessRecord(owner, owner.ID) {
		t.Fatal("owner must be allowed to access its own record")
	}
}

func TestCanAccessRecord_NonOwnerBeneficiaryDenied(t *testing.T) {
	other := otherBeneficiaryActor()
	ownerID := ownerActor().ID

	if services.CanAccessRecord(other, ownerID) {
		t.Fatal("non-owner Beneficiary must be denied access to another user's record")
	}
}

func TestCanAccessRecord_NonOwnerDonorDenied(t *testing.T) {
	other := otherDonorActor()
	ownerID := ownerActor().ID

	if services.CanAccessRecord(other, ownerID) {
		t.Fatal("non-owner Donor must be denied access to another user's record")
	}
}

func TestCanAccessRecord_PrivilegedRolesAllowed(t *testing.T) {
	ownerID := ownerActor().ID

	for _, role := range []string{
		models.RoleSuperAdmin,
		models.RoleAdmin,
		models.RoleManager,
		models.RoleStaff,
		models.RoleVolunteer,
	} {
		actor := services.Actor{ID: uuid.MustParse("00000000-0000-0000-0000-00000000bb01"), Role: role}
		if !services.CanAccessRecord(actor, ownerID) {
			t.Errorf("privileged role %q must retain legitimate cross-record access", role)
		}
	}
}

func TestCanAccessRecord_StudentIsNotPrivileged(t *testing.T) {
	actor := services.Actor{ID: uuid.MustParse("00000000-0000-0000-0000-00000000bb02"), Role: models.RoleStudent}
	ownerID := ownerActor().ID

	if services.CanAccessRecord(actor, ownerID) {
		t.Fatal("Student must be restricted to its own records")
	}
}

func TestCanAccessRecord_UnknownIdentityDenied(t *testing.T) {
	if services.CanAccessRecord(services.Actor{}, uuid.New()) {
		t.Fatal("actor without identity must never be authorized")
	}
}

func TestIsPrivilegedRole_Matrix(t *testing.T) {
	tests := []struct {
		role string
		want bool
	}{
		{models.RoleSuperAdmin, true},
		{models.RoleAdmin, true},
		{models.RoleManager, true},
		{models.RoleStaff, true},
		{models.RoleVolunteer, true},
		{models.RoleDonor, false},
		{models.RoleBeneficiary, false},
		{models.RoleStudent, false},
		{"", false},
		{"Hacker", false},
	}

	for _, tt := range tests {
		if got := services.IsPrivilegedRole(tt.role); got != tt.want {
			t.Errorf("IsPrivilegedRole(%q) = %v, want %v", tt.role, got, tt.want)
		}
	}
}

func TestIsPrivilegedRole_CaseInsensitive(t *testing.T) {
	if !services.IsPrivilegedRole("  admin ") {
		t.Fatal("role matching must be case/space insensitive")
	}
}

func TestActorFromUser(t *testing.T) {
	id := uuid.New()
	user := &models.User{
		ID:   id,
		Role: models.Role{Name: models.RoleStaff},
	}

	actor, ok := services.ActorFromUser(user)
	if !ok {
		t.Fatal("valid user must yield an actor")
	}
	if actor.ID != id || actor.Role != models.RoleStaff {
		t.Fatalf("actor = %+v, want id=%s role=%s", actor, id, models.RoleStaff)
	}

	if _, ok := services.ActorFromUser(nil); ok {
		t.Fatal("nil user must not yield an actor")
	}

	if _, ok := services.ActorFromUser(&models.User{}); ok {
		t.Fatal("user without identity must not yield an actor")
	}
}

func TestErrRecordAccessDenied_DistinctError(t *testing.T) {
	if !errors.Is(services.ErrRecordAccessDenied, services.ErrRecordAccessDenied) {
		t.Fatal("error identity must be preserved for handler mapping")
	}
	if errors.Is(services.ErrRecordAccessDenied, services.ErrInvalidLoanID) {
		t.Fatal("denial must not be conflated with validation errors")
	}
}
