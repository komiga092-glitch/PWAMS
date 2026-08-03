package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func Setup(router *gin.Engine) {

	router.GET("/", func(c *gin.Context) {

		c.JSON(http.StatusOK, gin.H{
			"application": "PWAMS",
			"message": "PWAMS API Running",
		})

	})

	router.GET("/health", func(c *gin.Context) {

		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
		})

	})

}