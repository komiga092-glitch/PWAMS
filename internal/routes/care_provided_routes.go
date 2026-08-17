package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
)

func RegisterCareProvidedRoutes(
	router *gin.Engine,
	handler *handlers.CareProvidedHandler,
	authMiddleware *middleware.AuthMiddleware,
) {
	protected := router.Group("/")
	protected.Use(
		authMiddleware.RequireAuth(),
	)

	protected.GET(
		"/care-provided/page",
		func(c *gin.Context) {
			c.HTML(http.StatusOK, "base", gin.H{
				"page_template": "care_provided_content",
				"title":         "Care Provided",
				"data":          []interface{}{},
			})
		},
	)

	protected.GET(
		"/care-provided",
		handler.List,
	)

	protected.POST(
		"/care-provided",
		handler.Create,
	)

	protected.GET(
		"/care-provided/:id",
		handler.GetByID,
	)

	protected.PUT(
		"/care-provided/:id",
		handler.Update,
	)

	protected.DELETE(
		"/care-provided/:id",
		handler.Delete,
	)
}
