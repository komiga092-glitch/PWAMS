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

// TestFailClosedRoutes_WithoutPermissionService verifies that sensitive route
// modules are NOT registered when the permission service is nil/unavailable.
// This is the fail-closed security principle: without authorization checks,
// privileged routes must 404 rather than become reachable with authentication
// alone.
func TestFailClosedRoutes_WithoutPermissionService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// All sensitive modules that must fail closed without a permission service.
	// Each entry specifies the registration function and the paths that must
	// not be reachable.
	modules := []struct {
		name     string
		register func(*gin.Engine)
		paths    []string
	}{
		{
			name: "persons",
			register: func(r *gin.Engine) {
				routes.RegisterPersonRoutes(
					r,
					handlers.NewPersonHandler(nil),
					middleware.NewAuthMiddleware(nil),
				)
			},
			paths: []string{
				"/persons/page",
				"/persons",
				"/persons/123",
			},
		},
		{
			name: "students",
			register: func(r *gin.Engine) {
				routes.RegisterStudentRoutes(
					r,
					handlers.NewStudentHandler(nil),
					middleware.NewAuthMiddleware(nil),
				)
			},
			paths: []string{
				"/students/page",
				"/students",
				"/students/123",
			},
		},
		{
			name: "donors",
			register: func(r *gin.Engine) {
				routes.RegisterDonorRoutes(
					r,
					handlers.NewDonorHandler(nil),
					middleware.NewAuthMiddleware(nil),
				)
			},
			paths: []string{
				"/donors/page",
				"/donors",
				"/donors/123",
			},
		},
		{
			name: "loans",
			register: func(r *gin.Engine) {
				routes.RegisterLoanRoutes(
					r,
					handlers.NewLoanHandler(nil),
					middleware.NewAuthMiddleware(nil),
				)
			},
			paths: []string{
				"/loans/page",
				"/loans",
				"/loans/123",
			},
		},
		{
			name: "loan-repayments",
			register: func(r *gin.Engine) {
				routes.RegisterLoanRepaymentRoutes(
					r,
					handlers.NewLoanRepaymentHandler(nil, nil),
					middleware.NewAuthMiddleware(nil),
				)
			},
			paths: []string{
				"/loan-repayments/page",
				"/loan-repayments",
				"/loan-repayments/123",
			},
		},
		{
			name: "revenue",
			register: func(r *gin.Engine) {
				routes.RegisterRevenueRoutes(
					r,
					handlers.NewRevenueHandler(nil, nil),
					middleware.NewAuthMiddleware(nil),
				)
			},
			paths: []string{
				"/revenue/page",
				"/revenue",
				"/revenue/123",
			},
		},
		{
			name: "messages",
			register: func(r *gin.Engine) {
				routes.RegisterMessageRoutes(
					r,
					handlers.NewMessageHandler(nil),
					handlers.NewMessageRecipientHandler(nil),
					middleware.NewAuthMiddleware(nil),
				)
			},
			paths: []string{
				"/messages/page",
				"/messages",
				"/messages/123",
			},
		},
		{
			name: "notifications",
			register: func(r *gin.Engine) {
				routes.RegisterNotificationRoutes(
					r,
					handlers.NewNotificationHandler(nil),
					middleware.NewAuthMiddleware(nil),
				)
			},
			paths: []string{
				"/notifications/page",
				"/notifications",
				"/notifications/123",
			},
		},
	}

	for _, mod := range modules {
		t.Run(mod.name, func(t *testing.T) {
			router := gin.New()
			mod.register(router)

			for _, path := range mod.paths {
				w := httptest.NewRecorder()
				router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))

				// Fail-closed: sensitive routes must NOT be registered when
				// the permission service is unavailable. A 404 confirms the
				// routes were never registered.
				if w.Code != http.StatusNotFound {
					t.Errorf("GET %s without permission service must be unregistered (404), got %d",
						path, w.Code)
				}
			}
		})
	}
}
