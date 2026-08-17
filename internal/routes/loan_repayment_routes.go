package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/handlers"
	"github.com/komiga092-glitch/pwams/internal/middleware"
)

func RegisterLoanRepaymentRoutes(
	router *gin.Engine,
	loanRepaymentHandler *handlers.LoanRepaymentHandler,
	authMiddleware *middleware.AuthMiddleware,
) {
	repayments := router.Group("/loan-repayments")

	repayments.Use(authMiddleware.RequireAuth())

	repayments.GET("/page", func(c *gin.Context) {
		c.HTML(http.StatusOK, "base", gin.H{
			"page_template": "loan_repayments_content",
			"title":         "Loan Repayments",
			"data":          []interface{}{},
		})
	})

	repayments.POST("", loanRepaymentHandler.Create)
	repayments.GET("", loanRepaymentHandler.List)
	repayments.GET("/:id", loanRepaymentHandler.GetByID)
	repayments.PATCH("/:id/pay", loanRepaymentHandler.Pay)
	repayments.PATCH("/:id/cancel", loanRepaymentHandler.Cancel)
}
