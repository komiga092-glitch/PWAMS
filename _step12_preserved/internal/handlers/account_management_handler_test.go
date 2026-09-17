package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/models"
)

// actorMiddleware simulates an authenticated actor with a cached effective
// permission set, mirroring middleware.RequireAuth + LoadPermissions without
// needing a database or session store.
func actorMiddleware(role string, permissions ...string) gin.HandlerFunc {
	user := &models.User{Username: "actor"}
	user.Role = models.Role{Name: role}

	return func(c *gin.Context) {
		c.Set("current_user", user)
		c.Set("permissions", permissions)
		c.Next()
	}
}

func performJSON(r http.Handler, method, target string, body string) *httptest.ResponseRecorder {
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestUnauthorizedUserGets403 verifies that the role-scoped management
// endpoints reject actors without the module permission (route-level gate,
// backend authorization — not merely hidden UI).
func TestUnauthorizedUserGets403(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name        string
		method      string
		path        string
		actorRole   string
		permissions []string
	}{
		{
			name:        "partner cannot view the admin management page",
			method:      http.MethodGet,
			path:        "/admins",
			actorRole:   models.RolePartner,
			permissions: []string{"users.view", "staff.view"},
		},
		{
			name:        "admin cannot list admins without admin.view permission",
			method:      http.MethodGet,
			path:        "/admins",
			actorRole:   models.RoleAdmin,
			permissions: []string{"users.view", "partner.view", "staff.view", "volunteer.view"},
		},
		{
			name:        "partner cannot list partners",
			method:      http.MethodGet,
			path:        "/partners",
			actorRole:   models.RolePartner,
			permissions: []string{"users.view", "staff.view"},
		},
		{
			// An Admin that lacks the (now Admin-held) admin.create
			// permission must still be rejected — the module permission is
			// the gate, and the hierarchy is the second line of defence.
			name:        "admin cannot create admins without admin.create permission",
			method:      http.MethodPost,
			path:        "/admins",
			actorRole:   models.RoleAdmin,
			permissions: []string{"users.create", "partner.create", "staff.create", "volunteer.create"},
		},
		{
			name:        "staff cannot create admins without admin.create permission",
			method:      http.MethodPost,
			path:        "/admins",
			actorRole:   models.RoleStaff,
			permissions: []string{"users.create", "donor.create", "beneficiary.create"},
		},
		{
			name:        "volunteer cannot create admins without admin.create permission",
			method:      http.MethodPost,
			path:        "/admins",
			actorRole:   models.RoleVolunteer,
			permissions: []string{"users.create", "donor.create"},
		},
		{
			name:        "donor cannot create admins without admin.create permission",
			method:      http.MethodPost,
			path:        "/admins",
			actorRole:   models.RoleDonor,
			permissions: []string{"donation.view"},
		},
		{
			name:        "beneficiary cannot create admins without admin.create permission",
			method:      http.MethodPost,
			path:        "/admins",
			actorRole:   models.RoleBeneficiary,
			permissions: []string{"aid.view_own", "loan.view_own"},
		},
		{
			name:        "student cannot create admins without admin.create permission",
			method:      http.MethodPost,
			path:        "/admins",
			actorRole:   models.RoleStudent,
			permissions: []string{"aid.view_own", "loan.view_own"},
		},
		{
			name:        "partner cannot delete admins",
			method:      http.MethodDelete,
			path:        "/admins/00000000-0000-0000-0000-000000000001",
			actorRole:   models.RolePartner,
			permissions: []string{"users.delete", "staff.delete"},
		},
		{
			name:        "volunteer cannot view staff management",
			method:      http.MethodGet,
			path:        "/staff",
			actorRole:   models.RoleVolunteer,
			permissions: []string{"users.view", "donor.view"},
		},
		{
			name:        "partner cannot open the admin management page directly",
			method:      http.MethodGet,
			path:        "/admins/page",
			actorRole:   models.RolePartner,
			permissions: []string{"users.view", "staff.view", "donor.view"},
		},
		{
			name:        "admin cannot open the system settings page directly",
			method:      http.MethodGet,
			path:        "/system-settings/page",
			actorRole:   models.RoleAdmin,
			permissions: []string{"users.view", "partner.view", "staff.view", "volunteer.view", "permission_management.view", "audit_logs.view"},
		},
		{
			name:        "admin cannot read the protected admin role permissions",
			method:      http.MethodGet,
			path:        "/permissions/roles/Admin",
			actorRole:   models.RoleAdmin,
			permissions: []string{"permission_management.view", "permission_management.edit"},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			handler := NewAccountManagementHandler(
				ManagedAccountConfig{Role: roleForPrefix(test.path), Label: "X", APIBase: "/" + test.path},
				nil, nil,
			)

			router := gin.New()
			router.Use(actorMiddleware(test.actorRole, test.permissions...))

			prefix := strings.TrimPrefix(test.path, "/")

			switch {
			case strings.HasPrefix(prefix, "admins"):
				router.GET("/admins", requireModulePermission("admin.view"), handler.List)
				router.GET("/admins/page", requireModulePermission("admin.view"), handler.Page)
				router.POST("/admins", requireModulePermission("admin.create"), handler.Create)
				router.GET("/admins/:id", requireModulePermission("admin.view"), handler.GetByID)
				router.DELETE("/admins/:id", requireModulePermission("admin.delete.approve"), handler.Delete)
			case strings.HasPrefix(prefix, "partners"):
				router.GET("/partners", requireModulePermission("partner.view"), handler.List)
				router.GET("/partners/page", requireModulePermission("partner.view"), handler.Page)
			case strings.HasPrefix(prefix, "staff"):
				router.GET("/staff", requireModulePermission("staff.view"), handler.List)
				router.GET("/staff/page", requireModulePermission("staff.view"), handler.Page)
			case strings.HasPrefix(prefix, "permissions/roles/Admin"):
				// The protected Admin role permission set must NOT be readable
				// by a non-Super-Admin even with permission_management.view.
				router.GET("/permissions/roles/Admin",
					requireModulePermission("permission_management.view"),
					func(c *gin.Context) {
						if !isSuperAdminActor(c) && !designerRoles["Admin"] {
							c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
								"success": false,
								"message": "This role's permission set can only be viewed by a Super Admin",
							})
							return
						}
						c.Status(http.StatusOK)
					})
			case strings.HasPrefix(prefix, "system-settings/page"):
				router.GET("/system-settings/page", requireModulePermission("system.settings.view"), handler.Page)
			}

			w := performJSON(router, test.method, test.path, "")
			if w.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusForbidden, w.Body.String())
			}
		})
	}
}

// requireModulePermission is the standalone route gate used in these tests,
// equivalent to middleware.RequirePermission against the cached set.
func requireModulePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		cached, _ := c.Get("permissions")
		perms, _ := cached.([]string)
		for _, p := range perms {
			if strings.EqualFold(p, permission) {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "You do not have permission to access this resource",
		})
	}
}

func roleForPrefix(prefix string) string {
	switch {
	case strings.HasPrefix(prefix, "admins"):
		return models.RoleAdmin
	case strings.HasPrefix(prefix, "partners"):
		return models.RolePartner
	case strings.HasPrefix(prefix, "staff"):
		return models.RoleStaff
	default:
		return models.RoleVolunteer
	}
}

func moduleViewPermission(prefix string) string {
	switch {
	case strings.HasPrefix(prefix, "admins"):
		return "admin.view"
	case strings.HasPrefix(prefix, "partners"):
		return "partner.view"
	case strings.HasPrefix(prefix, "staff"):
		return "staff.view"
	default:
		return "volunteer.view"
	}
}

// TestAdminCreateRouteGate verifies that the POST /admins route gate
// (admin.create) is satisfiable by both the Super Admin and Admin roles
// (multi-Admin) and stays closed for every other role. The full create flow
// (hierarchy enforcement, server-side role forcing, activation/temporary
// password setup) is covered by the DB-backed integration tests in
// admin_multi_account_integration_test.go.
func TestAdminCreateRouteGate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	allowed := []string{
		models.RoleSuperAdmin,
		models.RoleAdmin,
	}
	forbidden := []string{
		models.RolePartner,
		models.RoleStaff,
		models.RoleVolunteer,
		models.RoleDonor,
		models.RoleBeneficiary,
		models.RoleStudent,
	}

	buildRouter := func(role string, perms ...string) *gin.Engine {
		router := gin.New()
		router.Use(actorMiddleware(role, perms...))
		router.POST("/admins",
			requireModulePermission("admin.create"),
			func(c *gin.Context) { c.Status(http.StatusOK) },
		)
		return router
	}

	for _, adminRole := range allowed {
		t.Run(adminRole+" passes admin.create gate", func(t *testing.T) {
			w := performJSON(
				buildRouter(adminRole, "admin.create", "admin.view"),
				http.MethodPost,
				"/admins",
				"",
			)
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusOK, w.Body.String())
			}
		})
	}

	for _, role := range forbidden {
		t.Run(role+" blocked at admin.create gate", func(t *testing.T) {
			w := performJSON(
				buildRouter(role, "staff.create", "users.create"),
				http.MethodPost,
				"/admins",
				"",
			)
			if w.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusForbidden, w.Body.String())
			}
		})
	}
}
