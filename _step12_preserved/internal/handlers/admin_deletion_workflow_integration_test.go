package handlers

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// adminDeletionWorkflowHarness bundles the real services needed to exercise
// the supervised Admin account deletion workflow end to end.
type adminDeletionWorkflowHarness struct {
	db          *gorm.DB
	deletionSvc *services.AdminDeletionRequestService
}

func newAdminDeletionWorkflowHarness(t *testing.T) *adminDeletionWorkflowHarness {
	t.Helper()

	db, _ := adminIntegrationDB(t)

	userRepo := repository.NewUserRepository(db)
	userSvc := services.NewUserService(
		userRepo,
		repository.NewRoleRepository(db),
		repository.NewSessionRepository(db),
	)
	auditSvc := services.NewAuditLogService(repository.NewAuditLogRepository(db))

	return &adminDeletionWorkflowHarness{
		db: db,
		deletionSvc: services.NewAdminDeletionRequestService(
			repository.NewAdminDeletionRequestRepository(db),
			userRepo,
			userSvc,
			auditSvc,
		),
	}
}

// createTwoAdmins provisions two fresh Admin accounts through the real
// multi-Admin creation flow and returns them with their role populated.
func (h *adminDeletionWorkflowHarness) createTwoAdmins(
	t *testing.T,
	prefix string,
) (*models.User, *models.User) {
	t.Helper()

	handler := adminHandlerForTest(h.db)
	created := make([]string, 0, 2)
	for i := 0; i < 2; i++ {
		body := fmt.Sprintf(
			`{"email":%q,"full_name":"Admin %d","password_setup":"temporary_password","temporary_password":"TempPass123!"}`,
			fmt.Sprintf("%s-%d@pwams.local", prefix, i+1),
			i+1,
		)
		w := performJSON(
			adminCreateRouter(handler, models.RoleAdmin, "admin.create", "admin.view"),
			"POST", "/admins", body,
		)
		if w.Code != 201 {
			t.Fatalf("create admin #%d status=%d body=%s", i+1, w.Code, w.Body.String())
		}
		resp := parseCreateAdminResponse(t, []byte(w.Body.String()))
		created = append(created, resp.User.ID)
	}

	adminA := h.loadUser(t, created[0], models.RoleAdmin)
	adminB := h.loadUser(t, created[1], models.RoleAdmin)
	return adminA, adminB
}

func (h *adminDeletionWorkflowHarness) loadUser(t *testing.T, id, roleName string) *models.User {
	t.Helper()

	user, err := repository.NewUserRepository(h.db).FindByID(id)
	if err != nil {
		t.Fatalf("cannot load user %s: %v", id, err)
	}
	user.Role = models.Role{Name: roleName}
	return user
}

// superAdminActor loads the seeded Super Admin account.
func (h *adminDeletionWorkflowHarness) superAdminActor(t *testing.T) *models.User {
	t.Helper()

	var superAdmin models.User
	err := h.db.
		Joins("JOIN roles ON roles.id = users.role_id").
		Where("LOWER(roles.name) = ?", strings.ToLower(models.RoleSuperAdmin)).
		First(&superAdmin).Error
	if err != nil {
		t.Fatalf("cannot load seeded Super Admin: %v", err)
	}
	superAdmin.Role = models.Role{Name: models.RoleSuperAdmin}
	return &superAdmin
}

func (h *adminDeletionWorkflowHarness) userSoftDeleted(t *testing.T, id uuid.UUID) bool {
	t.Helper()

	var count int64
	if err := h.db.Unscoped().Model(&models.User{}).
		Where("id = ? AND deleted_at IS NOT NULL", id).
		Count(&count).Error; err != nil {
		t.Fatalf("cannot inspect soft-delete state: %v", err)
	}
	return count == 1
}

// TestIntegration_AdminDeletionWorkflow exercises the full request/approve
// and request/reject lifecycle against the real database and proves the
// privilege-escalation guards of the workflow.
func TestIntegration_AdminDeletionWorkflow(t *testing.T) {
	h := newAdminDeletionWorkflowHarness(t)

	prefix := "int-del-" + strings.ReplaceAll(uuid.NewString(), "-", "")
	cleanupAdminsByPrefix(t, h.db, prefix)

	adminA, adminB := h.createTwoAdmins(t, prefix)
	superAdmin := h.superAdminActor(t)

	// 1) An Admin may raise a deletion request against another Admin.
	req, err := h.deletionSvc.Create(adminA, models.CreateAdminDeletionRequest{
		TargetUserID: adminB.ID.String(),
		Reason:       "offboarding",
	})
	if err != nil {
		t.Fatalf("Admin must be able to request the deletion of another Admin: %v", err)
	}
	if req.Status != models.DeletionStatusPending {
		t.Fatalf("request status = %q, want pending", req.Status)
	}

	// 2) Only one pending request per target.
	if _, err := h.deletionSvc.Create(superAdmin, models.CreateAdminDeletionRequest{
		TargetUserID: adminB.ID.String(),
	}); !errors.Is(err, services.ErrDeletionDuplicate) {
		t.Errorf("duplicate request must fail with ErrDeletionDuplicate, got %v", err)
	}

	// 3) Four-eyes: the requester can never approve their own request.
	if _, err := h.deletionSvc.Approve(adminA, req.ID, ""); !errors.Is(err, services.ErrDeletionSelfApproval) {
		t.Errorf("requester approving own request must fail with ErrDeletionSelfApproval, got %v", err)
	}
	if _, err := h.deletionSvc.Reject(adminA, req.ID, ""); !errors.Is(err, services.ErrDeletionSelfApproval) {
		t.Errorf("requester rejecting own request must fail with ErrDeletionSelfApproval, got %v", err)
	}

	// 4) An Admin can never target a Super Admin account through the
	// workflow (the Super Admin role is out of reach for admin.*).
	if _, err := h.deletionSvc.Create(adminA, models.CreateAdminDeletionRequest{
		TargetUserID: superAdmin.ID.String(),
	}); !errors.Is(err, services.ErrDeletionTargetRole) {
		t.Errorf("Admin requesting deletion of a Super Admin must fail with ErrDeletionTargetRole, got %v", err)
	}

	// 5) Self-deletion requests are refused (removal of one's own account is
	// not a supported workflow operation).
	if _, err := h.deletionSvc.Create(adminA, models.CreateAdminDeletionRequest{
		TargetUserID: adminA.ID.String(),
	}); !errors.Is(err, services.ErrDeletionSelfRequest) {
		t.Errorf("self-deletion request must fail with ErrDeletionSelfRequest, got %v", err)
	}

	// 6) A different actor (Super Admin) approves — the target account is
	// soft-deleted and its sessions revoked via UserService.
	if _, err := h.deletionSvc.Approve(superAdmin, req.ID, "approved by system owner"); err != nil {
		t.Fatalf("approval by a different actor must succeed: %v", err)
	}
	if !h.userSoftDeleted(t, adminB.ID) {
		t.Error("approved request must soft-delete the target Admin account")
	}

	// 7) A resolved request can never be resolved again.
	if _, err := h.deletionSvc.Reject(superAdmin, req.ID, ""); !errors.Is(err, services.ErrDeletionAlreadyResolved) {
		t.Errorf("resolving a resolved request must fail with ErrDeletionAlreadyResolved, got %v", err)
	}
}

// TestIntegration_AdminDeletionRejectKeepsTargetActive proves the reject
// branch leaves the target account untouched.
func TestIntegration_AdminDeletionRejectKeepsTargetActive(t *testing.T) {
	h := newAdminDeletionWorkflowHarness(t)

	prefix := "int-rej-" + strings.ReplaceAll(uuid.NewString(), "-", "")
	cleanupAdminsByPrefix(t, h.db, prefix)

	adminA, adminB := h.createTwoAdmins(t, prefix)
	superAdmin := h.superAdminActor(t)

	req, err := h.deletionSvc.Create(adminA, models.CreateAdminDeletionRequest{
		TargetUserID: adminB.ID.String(),
		Reason:       "unfounded suspicion",
	})
	if err != nil {
		t.Fatalf("request creation failed: %v", err)
	}

	resolved, err := h.deletionSvc.Reject(superAdmin, req.ID, "not justified")
	if err != nil {
		t.Fatalf("rejection by a different actor must succeed: %v", err)
	}
	if resolved.Status != models.DeletionStatusRejected {
		t.Errorf("request status = %q, want rejected", resolved.Status)
	}
	if h.userSoftDeleted(t, adminB.ID) {
		t.Error("a rejected deletion request must not delete the target account")
	}
}
