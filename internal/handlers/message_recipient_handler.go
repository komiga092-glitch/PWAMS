package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
)

type MessageRecipientHandler struct {
	userRepo *repository.UserRepository
}

func NewMessageRecipientHandler(userRepo *repository.UserRepository) *MessageRecipientHandler {
	return &MessageRecipientHandler{userRepo: userRepo}
}

func (h *MessageRecipientHandler) List(c *gin.Context) {
	currentUser, ok := getCurrentUser(c)
	if !ok {
		return
	}

	users, _, err := h.userRepo.List("", "", 1, 100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Unable to retrieve message recipients",
		})
		return
	}

	recipients := make([]gin.H, 0, len(users))
	for _, user := range users {
		if user.ID == currentUser.ID || user.Status != models.UserStatusActive {
			continue
		}

		recipients = append(recipients, gin.H{
			"id":    user.ID,
			"name":  user.Username,
			"email": user.Email,
			"role":  user.Role.Name,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    recipients,
	})
}
