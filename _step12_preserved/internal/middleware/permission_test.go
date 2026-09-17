package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/models"
)

func TestRequirePermission_AllowsHeldPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("current_user", &models.User{})
		c.Set(permissionContextKey, []string{"donor.view", "donor.create"})
		c.Next()
	})
	// A nil permission service is safe here because the cached permission set
	// is consulted first and the service is never reached.
	router.GET("/donors", RequirePermission(nil, "donor.view"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/donors", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestRequirePermission_RejectsMissingPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("current_user", &models.User{})
		c.Set(permissionContextKey, []string{"donor.view"})
		c.Next()
	})
	router.DELETE("/donors/:id", RequirePermission(nil, "donor.delete"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodDelete, "/donors/123", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestRequirePermission_UnauthenticatedDenied(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/users", RequirePermission(nil, "users.view"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestRequireAllPermissions_RequiresEveryPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		held       []string
		wantStatus int
	}{
		{name: "all held", held: []string{"users.view", "users.edit"}, wantStatus: http.StatusOK},
		{name: "one missing", held: []string{"users.view"}, wantStatus: http.StatusForbidden},
		{name: "none held", held: []string{}, wantStatus: http.StatusForbidden},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("current_user", &models.User{})
				c.Set(permissionContextKey, test.held)
				c.Next()
			})
			router.GET("/admin",
				RequireAllPermissions(nil, "users.view", "users.edit"),
				func(c *gin.Context) {
					c.Status(http.StatusOK)
				})

			req := httptest.NewRequest(http.MethodGet, "/admin", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, test.wantStatus)
			}
		})
	}
}

func TestHasPermissionFromContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// With a populated context.
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set(permissionContextKey, []string{"donor.view"})
	if !HasPermissionFromContext(c, "donor.view") {
		t.Error("expected donor.view to be held")
	}
	if HasPermissionFromContext(c, "donor.delete") {
		t.Error("expected donor.delete to be missing")
	}

	// Case-insensitive matching.
	if !HasPermissionFromContext(c, "Donor.View") {
		t.Error("permission matching must be case-insensitive")
	}

	// Without a populated context.
	c2, _ := gin.CreateTestContext(httptest.NewRecorder())
	if HasPermissionFromContext(c2, "donor.view") {
		t.Error("empty context must not grant permissions")
	}
}

func TestLoadPermissions_NilServiceContinuesChain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	// Middleware must be registered before the route so the chain applies.
	router.Use(func(c *gin.Context) {
		c.Set("current_user", &models.User{})
		c.Next()
	})
	// The loader must tolerate a nil service (programming-error path)
	// without panicking or aborting the chain.
	router.Use(LoadPermissions(nil))
	router.GET("/check", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/check", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (LoadPermissions must continue the chain)", w.Code, http.StatusOK)
	}
}

func TestLoadPermissions_SkipsAnonymousRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(LoadPermissions(nil))
	router.GET("/anon", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/anon", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

// TestRequirePermission_ModuleGates verifies the granular permission gates
// wired across every module (donations, aid, care, loans, repayments,
// revenue, files, messages, notifications, reports, audit logs, persons,
// students) return 403 when the matching permission is not held, and 200
// when it is. This mirrors the backend route wiring where the permission is
// required independent of any role gate.
func TestRequirePermission_ModuleGates(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		permission string
	}{
		{"donation.view", "donation.view"},
		{"donation.create", "donation.create"},
		{"donation.delete", "donation.delete"},
		{"aid.view", "aid.view"},
		{"aid.approve", "aid.approve"},
		{"aid.delete", "aid.delete"},
		{"care.view", "care.view"},
		{"care.create", "care.create"},
		{"care.delete", "care.delete"},
		{"loan.view", "loan.view"},
		{"loan.approve", "loan.approve"},
		{"repayment.view", "repayment.view"},
		{"repayment.create", "repayment.create"},
		{"revenue.view", "revenue.view"},
		{"revenue.delete", "revenue.delete"},
		{"file.view", "file.view"},
		{"file.upload", "file.upload"},
		{"file.delete", "file.delete"},
		{"message.view", "message.view"},
		{"message.send", "message.send"},
		{"message.delete", "message.delete"},
		{"notification.view", "notification.view"},
		{"notification.create", "notification.create"},
		{"reports.view", "reports.view"},
		{"audit_logs.view", "audit_logs.view"},
		{"person.view", "person.view"},
		{"person.create", "person.create"},
		{"student.view", "student.view"},
		{"student.delete", "student.delete"},
		{"permission_management.view", "permission_management.view"},
		{"permission_management.edit", "permission_management.edit"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Held permission -> allowed.
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("current_user", &models.User{})
				c.Set(permissionContextKey, []string{test.permission})
				c.Next()
			})
			router.GET("/x", RequirePermission(nil, test.permission), func(c *gin.Context) {
				c.Status(http.StatusOK)
			})
			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("held status = %d, want %d", w.Code, http.StatusOK)
			}

			// Missing permission -> forbidden (403).
			router2 := gin.New()
			router2.Use(func(c *gin.Context) {
				c.Set("current_user", &models.User{})
				c.Set(permissionContextKey, []string{"unrelated.permission"})
				c.Next()
			})
			router2.GET("/x", RequirePermission(nil, test.permission), func(c *gin.Context) {
				c.Status(http.StatusOK)
			})
			req2 := httptest.NewRequest(http.MethodGet, "/x", nil)
			w2 := httptest.NewRecorder()
			router2.ServeHTTP(w2, req2)
			if w2.Code != http.StatusForbidden {
				t.Fatalf("missing status = %d, want %d", w2.Code, http.StatusForbidden)
			}
		})
	}
}
