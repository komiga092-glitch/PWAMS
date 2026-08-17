package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

type NotificationHandler struct {
	notificationService *services.NotificationService
}

func NewNotificationHandler(
	notificationService *services.NotificationService,
) *NotificationHandler {
	return &NotificationHandler{
		notificationService: notificationService,
	}
}

// List returns notifications belonging to the current user.
func (h *NotificationHandler) List(c *gin.Context) {
	currentUserValue, exists := c.Get("current_user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Authentication required",
		})
		return
	}

	currentUser, ok := currentUserValue.(*models.User)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Invalid authentication context",
		})
		return
	}

	notifications, err := h.notificationService.ListForUser(currentUser.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Unable to retrieve notifications",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"notifications": notifications,
	})
}

// GetByID returns one notification belonging to the current user.
func (h *NotificationHandler) GetByID(c *gin.Context) {
	id := c.Param("id")

	currentUserValue, exists := c.Get("current_user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Authentication required",
		})
		return
	}

	currentUser, ok := currentUserValue.(*models.User)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Invalid authentication context",
		})
		return
	}

	notification, err := h.notificationService.GetByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotificationNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Notification not found",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Unable to retrieve notification",
		})
		return
	}

	if notification.UserID != currentUser.ID {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Notification not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"notification": notification,
	})
}

// MarkAsRead marks the current user's notification as read.
func (h *NotificationHandler) MarkAsRead(c *gin.Context) {
	id := c.Param("id")

	currentUserValue, exists := c.Get("current_user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Authentication required",
		})
		return
	}

	currentUser, ok := currentUserValue.(*models.User)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Invalid authentication context",
		})
		return
	}

	err := h.notificationService.MarkAsRead(id, currentUser.ID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidNotification):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid notification ID",
			})

		case errors.Is(err, repository.ErrNotificationNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Notification not found",
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to mark notification as read",
			})
		}

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Notification marked as read",
	})
}

// Delete deletes the current user's notification.
func (h *NotificationHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	currentUserValue, exists := c.Get("current_user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Authentication required",
		})
		return
	}

	currentUser, ok := currentUserValue.(*models.User)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Invalid authentication context",
		})
		return
	}

	err := h.notificationService.Delete(id, currentUser.ID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidNotification):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid notification ID",
			})

		case errors.Is(err, repository.ErrNotificationNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Notification not found",
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to delete notification",
			})
		}

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Notification deleted successfully",
	})
}

// Create creates a notification for a user.
func (h *NotificationHandler) Create(c *gin.Context) {
	var request struct {
		UserID  string `json:"user_id" binding:"required"`
		Title   string `json:"title" binding:"required"`
		Message string `json:"message" binding:"required"`
		Type    string `json:"type" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid notification data",
		})
		return
	}

	userID, err := uuid.Parse(request.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid user ID",
		})
		return
	}

	notification := &models.Notification{
		UserID:  userID,
		Title:   request.Title,
		Message: request.Message,
		Type:    request.Type,
	}

	if err := h.notificationService.Create(notification); err != nil {
		if errors.Is(err, services.ErrInvalidNotification) {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid notification data",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Unable to create notification",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success":      true,
		"message":      "Notification created successfully",
		"notification": notification,
	})
}
