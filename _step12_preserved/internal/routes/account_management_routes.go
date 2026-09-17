package routes

import (
	"log"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// registerManagedAccountGroup wires one role-scoped account-management
// module (admins / partners / staff / volunteers). Every route is gated by
// the role's granular permission in addition to authentication, and the
// service layer enforces the role hierarchy independently of these gates.
//
// Because admin.* permissions are granted to the Super Admin and Admin roles
// (the Admin role manages other Admin accounts — multi-Admin support), the
// Admin Management module is reachable by Super Admin and Admin at the
// middleware level; the service layer still enforces that an Admin can never
// create or manage a Super Admin account.
//
// Fail-closed rule: a nil permission service is a mandatory security
// dependency failure. Privileged routes are NEVER registered without their
// permission gates — a nil service leaves the module unregistered (404)
// instead of exposing privileged mutations to any authenticated user.
// Production wiring additionally fails startup on a nil service
// (cmd/server/main.go), so registration with nil is a defensive last line.
func registerManagedAccountGroup(
	router *gin.Engine,
	permSvc *services.PermissionService,
	authMiddleware *middleware.AuthMiddleware,
	handler *handlers.AccountManagementHandler,
	prefix string,
) {
	if permSvc == nil {
		// Fail closed: without a permission service the granular gates
		// cannot be evaluated, so none of this module's routes are
		// registered. Registering them with authentication only would be
		// fail-open behaviour and is never acceptable for privileged
		// account-management endpoints.
		log.Printf("routes: permission service unavailable — /%s module NOT registered (fail-closed)", prefix)
		return
	}

	group := router.Group(prefix)
	group.Use(authMiddleware.RequireAuth())
	group.Use(middleware.LoadPermissions(permSvc))

	group.GET("/page",
		middleware.RequirePermission(permSvc, prefixViewPermission(prefix)),
		handler.Page)
	group.GET("",
		middleware.RequirePermission(permSvc, prefixViewPermission(prefix)),
		handler.List)
	group.GET("/:id",
		middleware.RequirePermission(permSvc, prefixViewPermission(prefix)),
		handler.GetByID)

	// Creating an Admin or Manager account is governed by the
	// ROLE HIERARCHY alone (final rule: "+ Add Admin" and "+ Add Manager"
	// need no separate create permission). The role gate admits only the
	// roles the hierarchy authorises; handler.Create and
	// UserService.CreateManagedAccount re-validate the hierarchy
	// independently (defence in depth). Staff and Volunteer creation keeps
	// its granular staff.create / volunteer.create gate.
	if moduleCreationIsRoleGated(prefix) {
		group.POST("",
			middleware.RequireAnyRole(models.RoleSuperAdmin, models.RoleAdmin),
			handler.Create)
	} else {
		group.POST("",
			middleware.RequirePermission(permSvc, prefixCreatePermission(prefix)),
			handler.Create)
	}

	group.PUT("/:id",
		middleware.RequirePermission(permSvc, prefixEditPermission(prefix)),
		handler.Update)
	// Status changes additionally require the direction-specific permission
	// (admin.activate / admin.deactivate / partner.* for Manager accounts) inside the handler.
	group.PATCH("/:id/status",
		middleware.RequirePermission(permSvc, prefixEditPermission(prefix)),
		handler.UpdateStatus)
	group.PATCH("/:id/password",
		middleware.RequirePermission(permSvc, "users.reset_password"),
		handler.ResetPassword)

	// Delete is only registered for roles whose catalog defines a delete
	// permission (admin.delete.approve, partner.delete, staff.delete).
	// Volunteers have no delete permission; deactivation is the supported
	// removal. Deleting an Admin account additionally requires the Super
	// Admin role on the direct route: a normal Admin can never delete
	// another Admin through this endpoint — that path is the supervised
	// /admins/deletion-requests request/approve/reject workflow
	// (admin.delete.request raises it, admin.delete.approve executes only
	// after another Admin approves). The workflow routes live in
	// RegisterAdminDeletionRequestRoutes under the same prefix.
	if handler.Config().Deletable {
		if modulePermissionNamespace(prefix) == "admin" {
			group.DELETE("/:id",
				middleware.RequireRole(models.RoleSuperAdmin),
				middleware.RequirePermission(permSvc, prefixDeletePermission(prefix)),
				handler.Delete)
		} else {
			group.DELETE("/:id",
				middleware.RequirePermission(permSvc, prefixDeletePermission(prefix)),
				handler.Delete)
		}
	}
}

// modulePermissionNamespace maps the module URL prefix to the permission
// namespace of the managed role. The URL prefixes are plural (admins,
// partners, ...) while the catalog permission namespaces are singular
// (admin.view, partner.view, ...), so the mapping must singularise; using the
// raw prefix would produce permission names (e.g. "admins.view") that no role
// can ever hold and would lock the whole module behind an unsatisfiable gate.
func modulePermissionNamespace(prefix string) string {
	switch prefix {
	case "admins":
		return "admin"
	case "partners":
		return "partner"
	case "staff":
		return "staff"
	case "volunteers":
		return "volunteer"
	default:
		return strings.TrimSuffix(prefix, "s")
	}
}

// moduleCreationIsRoleGated reports whether the module's POST (create)
// endpoint is gated by the role hierarchy alone instead of a granular create
// permission. Per the final role model this is true for the Admin and
// Manager modules: an authorised Admin may add another Admin or a
// Manager without holding admin.create / partner.create. Editing,
// deactivating and deleting those accounts KEEP their granular permission
// gates.
func moduleCreationIsRoleGated(prefix string) bool {
	switch modulePermissionNamespace(prefix) {
	case "admin", "partner":
		return true
	default:
		return false
	}
}

func prefixViewPermission(prefix string) string {
	return modulePermissionNamespace(prefix) + ".view"
}
func prefixCreatePermission(prefix string) string {
	return modulePermissionNamespace(prefix) + ".create"
}
func prefixEditPermission(prefix string) string {
	return modulePermissionNamespace(prefix) + ".edit"
}

// prefixDeletePermission returns the route gate for the module's DELETE
// endpoint. For the Admin module the gate is admin.delete.approve — the
// deletion-execution authority requiredTargetPermission maps the "delete"
// action to (direct Admin deletions go through the same permission as
// workflow approvals; raising a request only needs admin.delete.request).
func prefixDeletePermission(prefix string) string {
	if modulePermissionNamespace(prefix) == "admin" {
		return "admin.delete.approve"
	}
	return modulePermissionNamespace(prefix) + ".delete"
}

// RegisterAccountManagementRoutes registers the four Step 2 management
// modules. Each module shares the AccountManagementHandler implementation
// with a fixed target role, so the role being managed is always decided by
// the server-side route, never by the client.
func RegisterAccountManagementRoutes(
	router *gin.Engine,
	authMiddleware *middleware.AuthMiddleware,
	permissionService *services.PermissionService,
	admins, partners, staff, volunteers *handlers.AccountManagementHandler,
) {
	// Admin Management — Super Admin and Admin (both hold admin.*;
	// the hierarchy still forbids an Admin from managing Super Admins).
	registerManagedAccountGroup(router, permissionService, authMiddleware, admins, "admins")

	// Manager Management — Admin and Super Admin.
	registerManagedAccountGroup(router, permissionService, authMiddleware, partners, "partners")

	// Staff Management — Admin, Super Admin, and Partner (the NGO owner
	// manages its own staff; the hierarchy restricts targets server-side).
	registerManagedAccountGroup(router, permissionService, authMiddleware, staff, "staff")

	// Volunteer Management — same actor set as staff. No delete endpoint:
	// the catalog defines no volunteer.delete permission.
	registerManagedAccountGroup(router, permissionService, authMiddleware, volunteers, "volunteers")
}
