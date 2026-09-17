package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/config"
	"github.com/komiga092-glitch/pwams/internal/database"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// adminIntegrationDB connects to the configured PostgreSQL database and brings
// it to the canonical state (migrations + role seed + permission seed + super
// admin seed). When no database is reachable the tests are skipped so the
// suite still runs in a plain `go test ./...` environment; CI provides a
// database service.
func adminIntegrationDB(t *testing.T) (*gorm.DB, *config.Config) {
	t.Helper()

	_ = godotenv.Load("../../.env")

	cfg, err := config.Load()
	if err != nil {
		t.Skipf("admin integration test skipped (configuration unavailable): %v", err)
	}

	db, err := database.Connect(cfg)
	if err != nil {
		t.Skipf("admin integration test skipped (database unavailable): %v", err)
	}

	if err := database.Migrate(db); err != nil {
		t.Fatalf("admin integration test migration failed: %v", err)
	}
	if err := database.SeedDefaultRoles(db); err != nil {
		t.Fatalf("admin integration test role seed failed: %v", err)
	}
	permSvc := services.NewPermissionService(
		repository.NewPermissionRepository(db),
		repository.NewRoleRepository(db),
		repository.NewAuditLogRepository(db),
	)
	if err := permSvc.SeedDefaults(); err != nil {
		t.Fatalf("admin integration test permission seed failed: %v", err)
	}
	if err := database.SeedSuperAdmin(db, cfg); err != nil {
		t.Fatalf("admin integration test super-admin seed failed: %v", err)
	}

	return db, cfg
}

// adminHandlerForTest builds the real AccountManagementHandler for the Admin
// module backed by the real user / audit-log services (real repositories).
func adminHandlerForTest(db *gorm.DB) *AccountManagementHandler {
	userSvc := services.NewUserService(
		repository.NewUserRepository(db),
		repository.NewRoleRepository(db),
		repository.NewSessionRepository(db),
	)
	auditSvc := services.NewAuditLogService(repository.NewAuditLogRepository(db))

	return NewAccountManagementHandler(
		ManagedAccountConfig{
			Role:      models.RoleAdmin,
			Label:     "Admin",
			APIBase:   "/admins",
			Deletable: true,
		},
		userSvc,
		auditSvc,
	)
}

// adminCreateRouter registers the real POST /admins route (permission gate +
// handler) for an actor with the given role and permission set.
func adminCreateRouter(handler *AccountManagementHandler, role string, permissions ...string) *gin.Engine {
	router := gin.New()
	router.Use(actorMiddleware(role, permissions...))
	router.POST("/admins", requireModulePermission("admin.create"), handler.Create)
	return router
}

type createAdminResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	User    struct {
		ID   string `json:"id"`
		Role string `json:"role"`
	} `json:"user"`
}

func parseCreateAdminResponse(t *testing.T, body []byte) createAdminResponse {
	t.Helper()
	var resp createAdminResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("cannot decode create-admin response %q: %v", string(body), err)
	}
	return resp
}

// cleanupAdminsByPrefix removes every user whose email starts with prefix,
// along with the audit logs that reference them. The cleanup runs immediately
// (so leftover accounts from a previous run that used the same prefix do not
// leak into this test) and is also registered with t.Cleanup (so accounts
// created during this test are removed afterwards). Running it twice is safe
// because the immediate pass leaves nothing for the cleanup pass to find.
func cleanupAdminsByPrefix(t *testing.T, db *gorm.DB, prefix string) {
	t.Helper()
	prefix = strings.ToLower(strings.TrimSpace(prefix))

	doCleanup := func() {
		var ids []string
		_ = db.Model(&models.User{}).
			Where("LOWER(email) LIKE ?", prefix+"%").
			Pluck("id", &ids).Error
		if len(ids) > 0 {
			// Admin deletion requests reference users (target / requester /
			// responder) through foreign keys, so they must be removed before
			// the users themselves or the hard delete below would fail and
			// leave orphaned rows that break later AutoMigrate FK creation.
			_ = db.Unscoped().Where("target_user_id IN ?", ids).Delete(&models.AdminDeletionRequest{}).Error
			_ = db.Unscoped().Where("requester_id IN ?", ids).Delete(&models.AdminDeletionRequest{}).Error
			_ = db.Unscoped().Where("responded_by_id IN ?", ids).Delete(&models.AdminDeletionRequest{}).Error
			_ = db.Where("entity_id IN ?", ids).Delete(&models.AuditLog{}).Error
			_ = db.Where("user_id IN ?", ids).Delete(&models.AuditLog{}).Error
		}
		_ = db.Unscoped().Where("LOWER(email) LIKE ?", prefix+"%").Delete(&models.User{}).Error
	}

	doCleanup()
	t.Cleanup(doCleanup)
}

// cleanupAllAdmins parks every pre-existing Admin account for the duration of
// the Last Active Admin protection tests and restores it afterwards. Those
// tests assert on the absolute active-admin count, so the slate must be clean
// while they run — but pre-existing accounts belong to the operator's data,
// so they are never destroyed: park() demotes them to a disposable role with
// a throwaway identity (freeing the unique email/username indexes and
// removing them from the active-admin count), and the t.Cleanup pass puts
// each one back exactly as it was found. Admin accounts the tests themselves
// created (absent from the snapshot) are demoted permanently, and snapshotted
// accounts carrying test-fixture emails (@pwams.local / cleanup-*) are left
// demoted instead of restored so crashed runs do not accumulate test Admins.
// The park runs immediately; the demote+restore pass is registered with
// t.Cleanup.
func cleanupAllAdmins(t *testing.T, db *gorm.DB) {
	t.Helper()

	// Resolve a disposable non-Admin role. Admin accounts are reassigned to
	// this role rather than deleted: the users table is referenced by many
	// OnDelete:RESTRICT foreign keys (persons, donations, messages,
	// file_uploads, password_reset_tokens, ...), so deleting an Admin that
	// created any related record would fail.
	var disposableRole models.Role
	if err := db.Where("LOWER(name) = ?", strings.ToLower(models.RoleDonor)).First(&disposableRole).Error; err != nil {
		t.Fatalf("cannot resolve disposable role: %v", err)
	}

	type parkedAdmin struct {
		ID       uuid.UUID
		RoleID   uuid.UUID
		Email    string
		Username string
		Status   string
	}

	// isTestEmail reports whether the email belongs to the test fixture
	// namespace (createActiveAdmin / cleanup leftovers), never to operator
	// data.
	isTestEmail := func(email string) bool {
		e := strings.ToLower(strings.TrimSpace(email))
		return strings.HasSuffix(e, "@pwams.local") || strings.HasPrefix(e, "cleanup-")
	}

	mangle := func(id uuid.UUID, i int) {
		dummyEmail := fmt.Sprintf("cleanup-%s-%d@pwams.local", id.String()[:8], i)
		dummyUsername := fmt.Sprintf("cleanup_%s_%d", id.String()[:8], i)
		_ = db.Unscoped().Model(&models.User{}).
			Where("id = ?", id).
			Updates(map[string]interface{}{
				"role_id":  disposableRole.ID,
				"email":    dummyEmail,
				"username": dummyUsername,
			}).Error
	}

	// Snapshot every current Admin (Unscoped so soft-deleted rows — which
	// still occupy their email/username unique indexes — are included).
	var parked []parkedAdmin
	if err := db.Unscoped().Model(&models.User{}).
		Select("users.id", "users.role_id", "users.email", "users.username", "users.status").
		Joins("JOIN roles ON roles.id = users.role_id").
		Where("LOWER(roles.name) = ?", strings.ToLower(models.RoleAdmin)).
		Find(&parked).Error; err != nil {
		t.Fatalf("cannot snapshot Admin accounts: %v", err)
	}
	parkedIDs := make([]uuid.UUID, 0, len(parked))
	for _, a := range parked {
		parkedIDs = append(parkedIDs, a.ID)
	}

	// Park: remove every snapshot Admin from the count for the test run.
	for i, a := range parked {
		mangle(a.ID, i)
	}

	t.Cleanup(func() {
		// Demote the accounts the tests created (anything that is an Admin
		// now but was not in the snapshot).
		var created []models.User
		q := db.Unscoped().
			Select("users.id").
			Joins("JOIN roles ON roles.id = users.role_id").
			Where("LOWER(roles.name) = ?", strings.ToLower(models.RoleAdmin))
		if len(parkedIDs) > 0 {
			q = q.Where("users.id NOT IN ?", parkedIDs)
		}
		if err := q.Find(&created).Error; err == nil {
			for i, admin := range created {
				mangle(admin.ID, i)
			}
		}

		// Restore: snapshotted operator accounts go back exactly as found;
		// snapshotted test-fixture accounts are left demoted.
		for _, a := range parked {
			if isTestEmail(a.Email) {
				continue
			}
			_ = db.Unscoped().Model(&models.User{}).
				Where("id = ?", a.ID).
				Updates(map[string]interface{}{
					"role_id":  a.RoleID,
					"email":    a.Email,
					"username": a.Username,
					"status":   a.Status,
				}).Error
		}
	})
}

// TestIntegration_SuperAdminCanCreateAdmin proves (via the real route +
// handler + service + database) that a Super Admin can create an Admin
// account through POST /admins.
func TestIntegration_SuperAdminCanCreateAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, _ := adminIntegrationDB(t)
	handler := adminHandlerForTest(db)

	prefix := "int-sa-" + strings.ReplaceAll(uuid.NewString(), "-", "")
	cleanupAdminsByPrefix(t, db, prefix)

	body := fmt.Sprintf(
		`{"email":%q,"full_name":"Super-Created Admin","password_setup":"temporary_password","temporary_password":"TempPass123!"}`,
		prefix+"@pwams.local",
	)
	w := performJSON(
		adminCreateRouter(handler, models.RoleSuperAdmin, "admin.create", "admin.view"),
		http.MethodPost,
		"/admins",
		body,
	)
	if w.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	resp := parseCreateAdminResponse(t, w.Body.Bytes())
	if resp.User.Role != models.RoleAdmin {
		t.Fatalf("created user role = %q, want %q", resp.User.Role, models.RoleAdmin)
	}
}

// TestIntegration_AdminCanCreateAdmin proves that an Admin can create another
// Admin account through POST /admins (multi-Admin support).
func TestIntegration_AdminCanCreateAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, _ := adminIntegrationDB(t)
	handler := adminHandlerForTest(db)

	prefix := "int-adm-" + strings.ReplaceAll(uuid.NewString(), "-", "")
	cleanupAdminsByPrefix(t, db, prefix)

	body := fmt.Sprintf(
		`{"email":%q,"full_name":"Admin-Created Admin","password_setup":"temporary_password","temporary_password":"TempPass123!"}`,
		prefix+"@pwams.local",
	)
	w := performJSON(
		adminCreateRouter(handler, models.RoleAdmin, "admin.create", "admin.view"),
		http.MethodPost,
		"/admins",
		body,
	)
	if w.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	resp := parseCreateAdminResponse(t, w.Body.Bytes())
	if resp.User.Role != models.RoleAdmin {
		t.Fatalf("created user role = %q, want %q", resp.User.Role, models.RoleAdmin)
	}
}

// TestIntegration_AdminCannotCreateSuperAdmin_EscalationForced proves two
// things at once: (1) a malicious payload carrying role=Super Admin on
// POST /admins is forced server-side to Admin (never creates a Super Admin),
// and (2) the service layer independently rejects an Admin trying to create a
// Super Admin even when the request is crafted to bypass the endpoint forcing.
func TestIntegration_AdminCannotCreateSuperAdmin_EscalationForced(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, _ := adminIntegrationDB(t)
	handler := adminHandlerForTest(db)

	prefix := "int-esc-" + strings.ReplaceAll(uuid.NewString(), "-", "")
	cleanupAdminsByPrefix(t, db, prefix)

	// The handler must ignore the malicious role field and create an Admin.
	body := fmt.Sprintf(
		`{"email":%q,"full_name":"Escalation Attempt","password_setup":"temporary_password","temporary_password":"TempPass123!","role":"Super Admin"}`,
		prefix+"@pwams.local",
	)
	w := performJSON(
		adminCreateRouter(handler, models.RoleAdmin, "admin.create", "admin.view"),
		http.MethodPost,
		"/admins",
		body,
	)
	if w.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	resp := parseCreateAdminResponse(t, w.Body.Bytes())
	if resp.User.Role != models.RoleAdmin {
		t.Fatalf("escalation attempt created role %q, want %q (role must be forced server-side)", resp.User.Role, models.RoleAdmin)
	}

	// Independent service-level guard: bypassing the endpoint forcing is still
	// rejected by the role hierarchy.
	userSvc := services.NewUserService(
		repository.NewUserRepository(db),
		repository.NewRoleRepository(db),
		repository.NewSessionRepository(db),
	)
	_, err := userSvc.CreateManagedAccount(models.RoleAdmin, models.CreateManagedAccountRequest{
		Email:    "should-never-exist-" + strings.ReplaceAll(uuid.NewString(), "-", "") + "@pwams.local",
		FullName: "Never Created",
		Role:     models.RoleSuperAdmin,
	})
	if !errors.Is(err, services.ErrRoleHierarchyViolation) {
		t.Fatalf("Admin->Super Admin service call error = %v, want ErrRoleHierarchyViolation", err)
	}
}

// TestIntegration_LowRolesCannotCreateAdmin proves that Partner, Staff,
// Volunteer, Donor, Beneficiary and Student cannot create Admin accounts.
// Each actor is given admin.create in its simulated permission set so the
// only remaining guard is the backend role hierarchy — proving the hierarchy,
// not merely the permission gate, forbids them.
func TestIntegration_LowRolesCannotCreateAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, _ := adminIntegrationDB(t)
	handler := adminHandlerForTest(db)

	for _, role := range []string{
		models.RolePartner,
		models.RoleStaff,
		models.RoleVolunteer,
		models.RoleDonor,
		models.RoleBeneficiary,
		models.RoleStudent,
	} {
		t.Run(role+" cannot create Admin", func(t *testing.T) {
			prefix := "int-low-" + strings.ReplaceAll(uuid.NewString(), "-", "")
			cleanupAdminsByPrefix(t, db, prefix)

			body := fmt.Sprintf(
				`{"email":%q,"full_name":"Should Fail","password_setup":"temporary_password","temporary_password":"TempPass123!"}`,
				prefix+"@pwams.local",
			)
			// Even with admin.create in the permission set, the hierarchy
			// guard in the handler must reject the actor.
			w := performJSON(
				adminCreateRouter(handler, role, "admin.create", "admin.view"),
				http.MethodPost,
				"/admins",
				body,
			)
			if w.Code != http.StatusForbidden {
				t.Fatalf("status=%d body=%s, want 403 (hierarchy must forbid %s from creating Admin)", w.Code, w.Body.String(), role)
			}
		})
	}
}

// TestIntegration_MultipleAdminsCoexist proves there is no singleton Admin
// restriction: several Admin accounts can be created and coexist in the
// database.
func TestIntegration_MultipleAdminsCoexist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, _ := adminIntegrationDB(t)
	handler := adminHandlerForTest(db)

	prefix := "int-multi-" + strings.ReplaceAll(uuid.NewString(), "-", "")
	cleanupAdminsByPrefix(t, db, prefix)

	const count = 3
	createdIDs := make([]string, 0, count)
	for i := 0; i < count; i++ {
		body := fmt.Sprintf(
			`{"email":%q,"full_name":"Admin %d","password_setup":"temporary_password","temporary_password":"TempPass123!"}`,
			fmt.Sprintf("%s-%d@pwams.local", prefix, i),
			i+1,
		)
		w := performJSON(
			adminCreateRouter(handler, models.RoleAdmin, "admin.create", "admin.view"),
			http.MethodPost,
			"/admins",
			body,
		)
		if w.Code != http.StatusCreated {
			t.Fatalf("create #%d status=%d body=%s", i+1, w.Code, w.Body.String())
		}
		resp := parseCreateAdminResponse(t, w.Body.Bytes())
		if resp.User.Role != models.RoleAdmin {
			t.Fatalf("create #%d role = %q, want Admin", i+1, resp.User.Role)
		}
		createdIDs = append(createdIDs, resp.User.ID)
	}

	// Every created account must still be an Admin row in the database.
	var countAdmin int64
	if err := db.Model(&models.User{}).
		Joins("JOIN roles ON roles.id = users.role_id").
		Where("LOWER(roles.name) = ?", strings.ToLower(models.RoleAdmin)).
		Where("users.id IN ?", createdIDs).
		Count(&countAdmin).Error; err != nil {
		t.Fatalf("failed to count coexisting admins: %v", err)
	}
	if countAdmin != count {
		t.Fatalf("coexisting admins = %d, want %d (multiple Admin accounts must coexist)", countAdmin, count)
	}
}

// TestIntegration_AdminRoleHoldsAdminPermissionsAfterMigration proves that the
// migration + permission seed leaves the Admin role holding the full admin.*
// permission set, which is what authorises an Admin to reach POST /admins.
func TestIntegration_AdminRoleHoldsAdminPermissionsAfterMigration(t *testing.T) {
	db, _ := adminIntegrationDB(t)

	var adminRole models.Role
	if err := db.Where("LOWER(name) = ?", strings.ToLower(models.RoleAdmin)).First(&adminRole).Error; err != nil {
		t.Fatalf("cannot resolve Admin role: %v", err)
	}

	names := permissionNamesForRole(t, db, adminRole.ID)
	for _, perm := range []string{
		"admin.view", "admin.create", "admin.edit",
		"admin.activate", "admin.deactivate",
		"admin.delete.request", "admin.delete.approve", "admin.delete.reject",
	} {
		if !containsStringCaseInsensitive(names, perm) {
			t.Errorf("Admin role must hold %q after migration+seed (multi-Admin), got %v", perm, names)
		}
	}
}

func permissionNamesForRole(t *testing.T, db *gorm.DB, roleID uuid.UUID) []string {
	t.Helper()
	var names []string
	if err := db.Table("permissions").
		Select("permissions.name").
		Joins("JOIN role_permissions ON role_permissions.permission_id = permissions.id").
		Where("role_permissions.role_id = ?", roleID).
		Scan(&names).Error; err != nil {
		t.Fatalf("failed to load role permissions: %v", err)
	}
	return names
}

func containsStringCaseInsensitive(haystack []string, needle string) bool {
	needle = strings.ToLower(strings.TrimSpace(needle))
	for _, s := range haystack {
		if strings.ToLower(strings.TrimSpace(s)) == needle {
			return true
		}
	}
	return false
}
