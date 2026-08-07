package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/constants"
	"github.com/komiga092-glitch/pwams/internal/models"
)

type errorResponseMapping struct {
	err     error
	status  int
	message string
}

func getCurrentUser(c *gin.Context) (*models.User, bool) {
	value, exists := c.Get("current_user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": constants.ErrAuthenticationRequired,
		})
		return nil, false
	}

	currentUser, ok := value.(*models.User)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": constants.ErrInvalidAuthContext,
		})
		return nil, false
	}

	return currentUser, true
}

func buildPagination(total int64, page int, pageSize int) gin.H {
	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}

	return gin.H{
		"page":        page,
		"page_size":   pageSize,
		"total_items": total,
		"total_pages": totalPages,
	}
}

func writeErrorResponse(
	c *gin.Context,
	err error,
	defaultStatus int,
	defaultMessage string,
	mappings ...errorResponseMapping,
) {
	for _, mapping := range mappings {
		if errors.Is(err, mapping.err) {
			c.JSON(mapping.status, gin.H{
				"success": false,
				"message": mapping.message,
			})
			return
		}
	}

	c.JSON(defaultStatus, gin.H{
		"success": false,
		"message": defaultMessage,
	})
}
