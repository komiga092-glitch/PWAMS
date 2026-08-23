package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
	"github.com/komiga092-glitch/pwams/internal/models"
)

func RegisterRevenueRoutes(router *gin.Engine, handler *handlers.RevenueHandler, authMiddleware *middleware.AuthMiddleware) {
	revenue := router.Group("/revenue")
	revenue.Use(authMiddleware.RequireAuth())
	revenue.Use(middleware.RequireAnyRole(models.RoleSuperAdmin, models.RoleAdmin))
	revenue.GET("/page", handler.Page)
	revenue.GET("", handler.List)
	revenue.GET("/summaries", handler.Summaries)
	revenue.GET("/:id", handler.GetByID)
	revenue.POST("", handler.Create)
	revenue.PUT("/:id", handler.Update)
	revenue.DELETE("/:id", handler.Delete)
}
