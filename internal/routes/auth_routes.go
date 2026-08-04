package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/komiga092-glitch/pwams/internal/handlers"
)

func RegisterAuthRoutes(
	router *gin.Engine,
	authHandler *handlers.AuthHandler,
) {
	router.POST("/login", authHandler.Login)
}
