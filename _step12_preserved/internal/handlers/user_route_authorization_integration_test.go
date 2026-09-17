package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/middleware"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// These integration tests enforce the FINAL delete/authorization policy on
// the generic /users routes:
//
//   - An Admin can delete/deactivate every NORMAL user directly
//     (Partner, Staff, Volunteer, Donor, Beneficiary, Student, ...).
//   - An Admin can never delete an Admin through the generic route —
//     Admin-to-Admin deletion must use the supervised two-person workflow.
//   - No role below Super Admin can delete or even VIEW a Super Admin
//     account through the generic route (no /users/:id bypass of the
//     /admins/:id boundary).
//   - A Super Admin keeps full authority over every account.
//
// They reuse the shared adminIntegrationDB harness; without a reachable
// database the tests skip so `go test ./...` still passes offline.

// userHandlerForTest builds the real UserHandler backed by real services.
func userHandlerForTest(db *gorm.DB) *UserHandler {
	userSvc := services.NewUserService(
		repository.NewUserRepository(db),
		repository.NewRoleRepository(db),
		repository.NewSessionRepository(db),
	)
	auditSvc := services.NewAuditLogService(repository.NewAuditLogRepository(db))

	return NewUserHandler(userSvc, auditSvc)
}

// userDeleteRouter mirrors the production wiring of DELETE /users/:id
// (role gate + permission gate + handler) for the given actor.
func userDeleteRouter(handler *UserHandler, role string, permissions ...string) *gin.Engine {
	router := gin.New()
	router.Use(actorMiddleware(role, permissions...))
	router.Use(middleware.RequireAnyRole(models.RoleSuperAdmin, models.RoleAdmin))
	router.DELETE("/users/:id", requireModulePermission("users.delete"), handler.Delete)
	return router
}

// userGetRouter mirrors the production wiring of GET /users/:id.
func userGetRouter(handler *UserHandler, role string, permissions ...string) *gin.Engine {
	router := gin.New()
	router.Use(actorMiddleware(role, permissions...))
	router.GET("/users/:id", requireModulePermission("users.view"), handler.GetByID)
	return router
}

// seedFixtureUser inserts a test user directly into the database with the
// given role and registers an email-prefix cleanup. Used to create target
// accounts (including fixture Super Admins, which no API can create).
func seedFixtureUser(t *testing.T, db *gorm.DB, roleName, emailPrefix string) *models.User {
	t.Helper()

	var role models.Role
	if err := db.Where("LOWER(name) = ?", strings.ToLower(roleName)).First(&role).Error; err != nil {
		t.Fatalf("cannot resolve role %s: %v", roleName, err)
	}

	// Keep identifiers short: users.username is varchar(50).
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	email := fmt.Sprintf("%s-%s@pwams.local", emailPrefix, suffix)

	// Register the prefix cleanup FIRST: its immediate pass clears leftovers
	// from previous runs, and the t.Cleanup pass removes this fixture at test
	// end. (Calling it after the insert would delete the fixture immediately.)
	cleanupAdminsByPrefix(t, db, emailPrefix)

	user := models.User{
		Username:     fmt.Sprintf("%s_%s", emailPrefix, suffix),
		Email:        email,
		FullName:     "Route Auth Fixture",
		PasswordHash: "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy",
		RoleID:       role.ID,
		Role:         role,
		Status:       models.UserStatusActive,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("cannot create fixture user (%s): %v", roleName, err)
	}

	return &user
}

// adminPermissionsForAuthTest is the Admin role's default permission set
// subset relevant to the account-management routes.
func adminPermissionsForAuthTest() []string {
	return []string{
		"users.view", "users.create", "users.edit", "users.delete",
		"users.change_role", "users.activate", "users.enable", "users.disable",
		"users.reset_password",
		"admin.view", "admin.create", "admin.edit",
		"admin.activate", "admin.deactivate",
		"admin.delete.request", "admin.delete.approve", "admin.delete.reject",
		"partner.view", "partner.create", "partner.edit", "partner.delete",
		"donor.view",
	}
}

// TestIntegration_AdminDeletesNormalUserViaUsersRoute proves the final delete
// policy: an Admin deletes a normal (Donor) user directly through the
// generic users route.
func TestIntegration_AdminDeletesNormalUserViaUsersRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, _ := adminIntegrationDB(t)

	donor := seedFixtureUser(t, db, models.RoleDonor, "wd-donor")
	handler := userHandlerForTest(db)

	w := performJSON(
		userDeleteRouter(handler, models.RoleAdmin, adminPermissionsForAuthTest()...),
		http.MethodDelete,
		"/users/"+donor.ID.String(),
		"",
	)
	if w.Code != http.StatusOK {
		t.Fatalf("Admin deleting a normal user must succeed, status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"success":true`) {
		t.Fatalf("unexpected response body: %s", w.Body.String())
	}

	// The target must be gone from the normal (non-deleted) user set.
	var count int64
	db.Model(&models.User{}).Where("id = ?", donor.ID).Count(&count)
	if count != 0 {
		t.Fatal("deleted Donor must no longer be part of the normal user set")
	}
}

// TestIntegration_AdminCannotDeleteAdminViaUsersRoute proves the four-eyes
// rule has no bypass: an Admin can never delete another Admin directly
// through the generic users route — the supervised workflow is required.
func TestIntegration_AdminCannotDeleteAdminViaUsersRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, _ := adminIntegrationDB(t)

	target := seedFixtureUser(t, db, models.RoleAdmin, "wd-admin-target")
	handler := userHandlerForTest(db)

	w := performJSON(
		userDeleteRouter(handler, models.RoleAdmin, adminPermissionsForAuthTest()...),
		http.MethodDelete,
		"/users/"+target.ID.String(),
		"",
	)
	if w.Code != http.StatusForbidden {
		t.Fatalf("Admin deleting an Admin via /users/:id must be forbidden, status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "supervised approval workflow") {
		t.Fatalf("response must point to the workflow, got: %s", w.Body.String())
	}

	var count int64
	db.Model(&models.User{}).Where("id = ?", target.ID).Count(&count)
	if count != 1 {
		t.Fatal("the targeted Admin account must remain untouched")
	}
}

// TestIntegration_AdminCannotDeleteSuperAdminViaUsersRoute proves a Super
// Admin account is never deletable through the generic users route by a
// non-Super-Admin actor (no bypass of the /admins/:id protection).
func TestIntegration_AdminCannotDeleteSuperAdminViaUsersRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, _ := adminIntegrationDB(t)

	superTarget := seedFixtureUser(t, db, models.RoleSuperAdmin, "wd-super-target")
	handler := userHandlerForTest(db)

	w := performJSON(
		userDeleteRouter(handler, models.RoleAdmin, adminPermissionsForAuthTest()...),
		http.MethodDelete,
		"/users/"+superTarget.ID.String(),
		"",
	)
	if w.Code != http.StatusForbidden {
		t.Fatalf("Admin deleting a Super Admin via /users/:id must be forbidden, status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), services.ErrCannotModifySuperAdmin.Error()) {
		t.Fatalf("response must carry the Super Admin protection message, got: %s", w.Body.String())
	}
}

// TestIntegration_AdminCannotViewSuperAdminViaUsersRoute proves a Super
// Admin account's details never leak through GET /users/:id to a non-Super-
// Admin actor (view-side bypass fix).
func TestIntegration_AdminCannotViewSuperAdminViaUsersRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, _ := adminIntegrationDB(t)

	superTarget := seedFixtureUser(t, db, models.RoleSuperAdmin, "wd-super-view")
	handler := userHandlerForTest(db)

	w := performJSON(
		userGetRouter(handler, models.RoleAdmin, "users.view", "admin.view"),
		http.MethodGet,
		"/users/"+superTarget.ID.String(),
		"",
	)
	if w.Code != http.StatusForbidden {
		t.Fatalf("Admin viewing a Super Admin via /users/:id must be forbidden, status=%d body=%s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), superTarget.Email) {
		t.Fatal("the Super Admin account details must never be leaked in the response")
	}
}

// TestIntegration_SuperAdminKeepsFullUsersRouteAuthority proves the guard
// did not over-restrict the system owner: a Super Admin can view a Super
// Admin account and delete a normal user through the same routes.
func TestIntegration_SuperAdminKeepsFullUsersRouteAuthority(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, _ := adminIntegrationDB(t)

	superTarget := seedFixtureUser(t, db, models.RoleSuperAdmin, "wd-super-self")
	donor := seedFixtureUser(t, db, models.RoleDonor, "wd-super-donor")
	handler := userHandlerForTest(db)

	router := gin.New()
	router.Use(actorMiddleware(models.RoleSuperAdmin, "users.view", "users.delete"))
	router.GET("/users/:id", requireModulePermission("users.view"), handler.GetByID)
	router.DELETE("/users/:id", middleware.RequireAnyRole(models.RoleSuperAdmin, models.RoleAdmin), requireModulePermission("users.delete"), handler.Delete)

	w := performJSON(router, http.MethodGet, "/users/"+superTarget.ID.String(), "")
	if w.Code != http.StatusOK {
		t.Fatalf("Super Admin viewing a Super Admin must succeed, status=%d body=%s", w.Code, w.Body.String())
	}

	w = performJSON(router, http.MethodDelete, "/users/"+donor.ID.String(), "")
	if w.Code != http.StatusOK {
		t.Fatalf("Super Admin deleting a normal user must succeed, status=%d body=%s", w.Code, w.Body.String())
	}
}

// TestIntegration_PartnerCannotDeleteViaUsersRoute proves the role gate: no
// operational role reaches the deletion handler even with crafted
// permission payloads.
func TestIntegration_PartnerCannotDeleteViaUsersRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, _ := adminIntegrationDB(t)

	donor := seedFixtureUser(t, db, models.RoleDonor, "wd-partner-donor")
	handler := userHandlerForTest(db)

	w := performJSON(
		userDeleteRouter(handler, models.RolePartner, "users.delete", "users.view"),
		http.MethodDelete,
		"/users/"+donor.ID.String(),
		"",
	)
	if w.Code != http.StatusForbidden {
		t.Fatalf("Partner must be rejected by the role gate, status=%d body=%s", w.Code, w.Body.String())
	}

	var count int64
	db.Model(&models.User{}).Where("id = ?", donor.ID).Count(&count)
	if count != 1 {
		t.Fatal("the target must remain untouched")
	}
}

// TestIntegration_SuperAdminRoleHoldsEveryCatalogPermission compares the
// Super Admin role's persisted permission set (as resolved for the seeded
// system-owner account after migrations + seeds) against the COMPLETE
// canonical permission catalog. Expected: missing permissions = 0. New
// permissions added to the catalog are picked up automatically because
// SeedDefaults always repairs the Super Admin role to the full catalog.
func TestIntegration_SuperAdminRoleHoldsEveryCatalogPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, _ := adminIntegrationDB(t)

	permSvc := services.NewPermissionService(
		repository.NewPermissionRepository(db),
		repository.NewRoleRepository(db),
		repository.NewAuditLogRepository(db),
	)

	var superAdminRole models.Role
	if err := db.Where("name = ?", models.RoleSuperAdmin).First(&superAdminRole).Error; err != nil {
		t.Fatalf("Super Admin role not found: %v", err)
	}

	// Resolve the seeded Super Admin account. User-level overrides never
	// grant protected permissions, so the role matrix is authoritative.
	var superAdminUser models.User
	if err := db.Where("role_id = ?", superAdminRole.ID).First(&superAdminUser).Error; err != nil {
		t.Fatalf("seeded Super Admin account not found: %v", err)
	}

	names, err := permSvc.GetUserPermissions(&superAdminUser)
	if err != nil {
		t.Fatalf("cannot load Super Admin permissions: %v", err)
	}

	held := make(map[string]bool, len(names))
	for _, n := range names {
		held[strings.ToLower(strings.TrimSpace(n))] = true
	}

	var missing []string
	for _, p := range services.AllPermissions() {
		if !held[p.Name] {
			missing = append(missing, p.Name)
		}
	}

	if len(missing) != 0 {
		t.Fatalf("Super Admin missing permissions = %d, want 0: %v", len(missing), missing)
	}
}
