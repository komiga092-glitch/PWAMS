package routes_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
	"github.com/komiga092-glitch/pwams/internal/routes"
)

// TestReportRoutes_FailClosedWithoutPermissionService pins the security
// posture of the reports module: without a working permission service the
// whole /reports group is NOT registered (fail-closed), so privileged report
// pages can never become reachable for every authenticated role by accident.
func TestReportRoutes_FailClosedWithoutPermissionService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	routes.RegisterReportRoutes(
		router,
		handlers.NewReportHandler(nil),
		middleware.NewAuthMiddleware(nil),
	)

	for _, path := range []string{
		"/reports/dashboard/page",
		"/reports/dashboard",
		"/reports/donations",
		"/reports/aid-requests",
		"/reports/export",
	} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusNotFound {
			t.Errorf("GET %s without permission service must be unregistered (404), got %d", path, w.Code)
		}
	}
}
