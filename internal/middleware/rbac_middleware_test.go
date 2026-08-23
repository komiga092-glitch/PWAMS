package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/komiga092-glitch/pwams/internal/models"
)

func TestRequireAnyRole(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		user       *models.User
		wantStatus int
	}{
		{name: "missing user", wantStatus: http.StatusUnauthorized},
		{name: "wrong role", user: &models.User{Role: models.Role{Name: models.RoleVolunteer}}, wantStatus: http.StatusForbidden},
		{name: "allowed role", user: &models.User{Role: models.Role{Name: models.RoleStaff}}, wantStatus: http.StatusOK},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			if test.user != nil {
				router.Use(func(c *gin.Context) {
					c.Set("current_user", test.user)
					c.Next()
				})
			}
			router.Use(RequireAnyRole(models.RoleSuperAdmin, models.RoleAdmin, models.RoleStaff))
			router.GET("/protected", func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			request := httptest.NewRequest(http.MethodGet, "/protected", nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
		})
	}
}
