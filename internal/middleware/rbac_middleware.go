package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/models"
)

// RequireRole allows access only to one specific role.
func RequireRole(requiredRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		value, exists := c.Get("current_user")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Authentication required",
			})
			return
		}

		user, ok := value.(*models.User)
		if !ok {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Invalid authentication context",
			})
			return
		}

		if user.Role.Name != requiredRole {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": "You do not have permission to access this resource",
			})
			return
		}

		c.Next()
	}
}

// RequireAnyRole allows access if the user has any one of the allowed roles.
func RequireAnyRole(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		value, exists := c.Get("current_user")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Authentication required",
			})
			return
		}

		user, ok := value.(*models.User)
		if !ok {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Invalid authentication context",
			})
			return
		}

		for _, role := range allowedRoles {
			if user.Role.Name == role {
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "You do not have permission to access this resource",
		})
	}
}
