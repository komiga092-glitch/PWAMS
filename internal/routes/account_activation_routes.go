package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/handlers"
)

func RegisterAccountActivationRoutes(
	router *gin.Engine,
	handler *handlers.AccountActivationHandler,
) {
	router.POST(
		"/request-account-activation",
		handler.RequestActivation,
	)

	router.POST(
		"/verify-account-activation-otp",
		handler.VerifyOTP,
	)

	router.POST(
		"/reactivate-account",
		handler.Reactivate,
	)
}
