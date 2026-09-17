package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/models"
)

func newTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	return c, w
}

// PageData must expose the permission set cached by middleware
// LoadPermissions so layouts can render permission-aware navigation.
func TestPageData_InjectsCachedPermissions(t *testing.T) {
	c, _ := newTestContext()
	user := &models.User{Username: "staff"}
	c.Set("current_user", user)
	c.Set("permissions", []string{"care.view", "Donor.View", "  donation.view  "})

	data := PageData(c, gin.H{"page_template": "dashboard_content"})

	perms, ok := data["user_permissions"].(map[string]bool)
	if !ok {
		t.Fatalf("user_permissions missing or wrong type: %T", data["user_permissions"])
	}
	if !perms["care.view"] {
		t.Error("expected care.view to be held")
	}
	if !perms["donor.view"] {
		t.Error("permission matching must be case-insensitive (Donor.View -> donor.view)")
	}
	if !perms["donation.view"] {
		t.Error("permission names must be trimmed")
	}
	if perms["care.delete"] {
		t.Error("permissions not held must not be present")
	}

	if data["current_user"] != user {
		t.Error("PageData must keep current_user in the template data")
	}
}

// When the permission middleware has not populated the cache the map must be
// empty (fail closed): permission-gated UI stays hidden rather than leaking.
func TestPageData_FailsClosedWithoutPermissionCache(t *testing.T) {
	c, _ := newTestContext()
	c.Set("current_user", &models.User{Username: "staff"})

	data := PageData(c, gin.H{})

	perms, ok := data["user_permissions"].(map[string]bool)
	if !ok {
		t.Fatalf("user_permissions missing or wrong type: %T", data["user_permissions"])
	}
	if len(perms) != 0 {
		t.Errorf("expected empty permission set, got %v", perms)
	}
}

// Anonymous requests must not receive any permissions.
func TestPageData_AnonymousGetsNoPermissions(t *testing.T) {
	c, _ := newTestContext()

	data := PageData(c, gin.H{})

	perms, ok := data["user_permissions"].(map[string]bool)
	if !ok {
		t.Fatalf("user_permissions missing or wrong type: %T", data["user_permissions"])
	}
	if len(perms) != 0 {
		t.Errorf("anonymous request must have no permissions, got %v", perms)
	}
	if _, hasUser := data["current_user"]; hasUser {
		t.Error("anonymous request must not carry a current_user")
	}
}
