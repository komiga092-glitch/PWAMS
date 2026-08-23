package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
	"github.com/komiga092-glitch/pwams/internal/models"
)

func RegisterDonorRoutes(
	router *gin.Engine,
	donorHandler *handlers.DonorHandler,
	authMiddleware *middleware.AuthMiddleware,
) {
	donors := router.Group("/donors")

	donors.Use(authMiddleware.RequireAuth())

	donors.Use(
		middleware.RequireAnyRole(
			models.RoleSuperAdmin,
			models.RoleAdmin,
			models.RoleStaff,
		),
	)

	// =========================
	// HTML PAGES
	// =========================

	// Donor list page
	donors.GET(
		"/page",
		donorHandler.Page,
	)

	// Donor view page
	donors.GET(
		"/:id/view",
		donorHandler.ViewPage,
	)

	// Donor edit page
	donors.GET(
		"/:id/edit",
		donorHandler.EditPage,
	)

	// =========================
	// API
	// =========================

	// List donors API
	donors.GET(
		"",
		donorHandler.List,
	)

	// Get donor API
	donors.GET(
		"/:id",
		donorHandler.GetByID,
	)

	// Create donor API
	donors.POST(
		"",
		donorHandler.Create,
	)

	// Update donor API
	donors.PUT(
		"/:id",
		donorHandler.Update,
	)

	// Update donor status API
	donors.PATCH(
		"/:id/status",
		donorHandler.UpdateStatus,
	)

	// Delete donor API
	donors.DELETE(
		"/:id",
		middleware.RequireAnyRole(
			models.RoleSuperAdmin,
			models.RoleAdmin,
		),
		donorHandler.Delete,
	)
}
