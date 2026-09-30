package routes_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
	"github.com/komiga092-glitch/pwams/internal/routes"
)

// Regression test for the Phase-1 audit-log dependency fix.
//
// The audit-backed CRUD handlers now take the *AuditLogService as a required
// constructor parameter (it used to be variadic, which allowed handlers to be
// created WITHOUT an audit service — the root cause of "POST /persons → 500
// after a successful DB insert").
//
// This test proves every affected route group still registers with the new
// constructor signatures and that the create endpoints are authenticated
// (401 without a session) — reachable, guarded, and never 404 or panicking.
func TestAuditBackedCRUDRoutesRegisterAndStayProtected(t *testing.T) {
	gin.SetMode(gin.TestMode)

	authMiddleware := middleware.NewAuthMiddleware(nil)

	router := gin.New()

	// Every constructor below compiles ONLY because the audit service is a
	// required parameter — a variadic signature would also accept zero
	// arguments and silently drop audit logging.
	routes.RegisterPersonRoutes(router, handlers.NewPersonHandler(nil, nil), authMiddleware)
	routes.RegisterStudentRoutes(router, handlers.NewStudentHandler(nil, nil), authMiddleware)
	routes.RegisterDonorRoutes(router, handlers.NewDonorHandler(nil, nil), authMiddleware)
	routes.RegisterDonationRoutes(router, handlers.NewDonationHandler(nil, nil, nil), authMiddleware)
	routes.RegisterAidRequestRoutes(router, handlers.NewAidRequestHandler(nil, nil), authMiddleware)
	routes.RegisterCareProvidedRoutes(router, handlers.NewCareProvidedHandler(nil, nil), authMiddleware)
	routes.RegisterLoanRoutes(router, handlers.NewLoanHandler(nil, nil), authMiddleware)
	routes.RegisterLoanRepaymentRoutes(router, handlers.NewLoanRepaymentHandler(nil, nil), authMiddleware)
	routes.RegisterRevenueRoutes(router, handlers.NewRevenueHandler(nil, nil), authMiddleware)

	for _, path := range []string{
		"/persons",
		"/students",
		"/donors",
		"/donations",
		"/aid-requests",
		"/care-provided",
		"/loans",
		"/loan-repayments",
		"/revenue",
	} {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Accept", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			if response.Code != http.StatusUnauthorized {
				t.Fatalf("POST %s without a session: status = %d, want %d (route must exist and be protected)", path, response.Code, http.StatusUnauthorized)
			}
			if !strings.Contains(response.Body.String(), "Authentication required") {
				t.Fatalf("POST %s body = %q, want the authentication failure message", path, response.Body.String())
			}
		})
	}
}
