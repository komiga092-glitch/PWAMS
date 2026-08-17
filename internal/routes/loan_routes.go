package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
)

func RegisterLoanRoutes(
	router *gin.Engine,
	loanHandler *handlers.LoanHandler,
	authMiddleware *middleware.AuthMiddleware,
) {
	loans := router.Group("/loans")

	loans.Use(authMiddleware.RequireAuth())

	loans.GET("/page", func(c *gin.Context) {
		c.HTML(http.StatusOK, "base", gin.H{
			"page_template": "loans_content",
			"title":         "Loans",
			"data":          []interface{}{},
		})
	})

	loans.POST("", loanHandler.Create)
	loans.GET("", loanHandler.List)
	loans.GET("/:id", loanHandler.GetByID)
	loans.PATCH("/:id/review", loanHandler.Review)
}
