package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func Setup(router *gin.Engine) {
	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "base", gin.H{
			"page_template": "home_content",
			"title":         "PWAMS",
		})
	})

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"application": "PWAMS",
			"message":     "PWAMS API Running",
			"status":      "healthy",
		})
	})
}
