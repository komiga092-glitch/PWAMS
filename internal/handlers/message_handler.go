package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

type MessageHandler struct {
	messageService *services.MessageService
}

func NewMessageHandler(messageService *services.MessageService) *MessageHandler {
	return &MessageHandler{messageService: messageService}
}

type createMessageRequest struct {
	RecipientID string `json:"recipient_id" binding:"required"`
	Subject     string `json:"subject" binding:"required"`
	Body        string `json:"body" binding:"required"`
}

func (h *MessageHandler) List(c *gin.Context) {
	h.listInbox(c)
}

func (h *MessageHandler) Inbox(c *gin.Context) {
	h.listInbox(c)
}

func (h *MessageHandler) Sent(c *gin.Context) {
	currentUser, ok := getCurrentUser(c)
	if !ok {
		return
	}

	page, pageSize, valid := messagePagination(c)
	if !valid {
		return
	}

	messages, total, err := h.messageService.ListSent(
		currentUser.ID.String(),
		page,
		pageSize,
	)
	if err != nil {
		writeMessageServiceError(c, err, "Unable to retrieve sent messages")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "Sent messages retrieved successfully",
		"data":       messages,
		"pagination": buildPagination(total, page, pageSize),
	})
}

func (h *MessageHandler) Unread(c *gin.Context) {
	currentUser, ok := getCurrentUser(c)
	if !ok {
		return
	}

	page, pageSize, valid := messagePagination(c)
	if !valid {
		return
	}

	messages, total, err := h.messageService.ListUnread(
		currentUser.ID.String(),
		page,
		pageSize,
	)
	if err != nil {
		writeMessageServiceError(c, err, "Unable to retrieve unread messages")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "Unread messages retrieved successfully",
		"data":       messages,
		"pagination": buildPagination(total, page, pageSize),
	})
}

func (h *MessageHandler) UnreadCount(c *gin.Context) {
	currentUser, ok := getCurrentUser(c)
	if !ok {
		return
	}

	count, err := h.messageService.GetUnreadCount(currentUser.ID.String())
	if err != nil {
		writeMessageServiceError(c, err, "Unable to count unread messages")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"message":      "Unread message count retrieved successfully",
		"unread_count": count,
	})
}

func (h *MessageHandler) GetByID(c *gin.Context) {
	currentUser, ok := getCurrentUser(c)
	if !ok {
		return
	}

	message, err := h.messageService.GetMessage(
		c.Param("id"),
		currentUser.ID.String(),
	)
	if err != nil {
		writeMessageServiceError(c, err, "Unable to retrieve message")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Message retrieved successfully",
		"data":    message,
	})
}

func (h *MessageHandler) Create(c *gin.Context) {
	currentUser, ok := getCurrentUser(c)
	if !ok {
		return
	}

	var request createMessageRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid message data",
		})
		return
	}

	created, err := h.messageService.SendMessage(
		currentUser.ID.String(),
		request.RecipientID,
		request.Subject,
		request.Body,
	)
	if err != nil {
		writeMessageServiceError(c, err, "Unable to send message")
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Message sent successfully",
		"data":    created,
	})
}

func (h *MessageHandler) MarkAsRead(c *gin.Context) {
	currentUser, ok := getCurrentUser(c)
	if !ok {
		return
	}

	err := h.messageService.MarkAsRead(
		c.Param("id"),
		currentUser.ID.String(),
	)
	if err != nil {
		writeMessageServiceError(c, err, "Unable to mark message as read")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Message marked as read",
	})
}

func (h *MessageHandler) Delete(c *gin.Context) {
	currentUser, ok := getCurrentUser(c)
	if !ok {
		return
	}

	err := h.messageService.DeleteMessage(
		c.Param("id"),
		currentUser.ID.String(),
	)
	if err != nil {
		writeMessageServiceError(c, err, "Unable to delete message")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Message deleted successfully",
	})
}

func (h *MessageHandler) listInbox(c *gin.Context) {
	currentUser, ok := getCurrentUser(c)
	if !ok {
		return
	}

	page, pageSize, valid := messagePagination(c)
	if !valid {
		return
	}

	messages, total, err := h.messageService.ListInbox(
		currentUser.ID.String(),
		page,
		pageSize,
	)
	if err != nil {
		writeMessageServiceError(c, err, "Unable to retrieve inbox messages")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "Inbox messages retrieved successfully",
		"data":       messages,
		"pagination": buildPagination(total, page, pageSize),
	})
}

func messagePagination(c *gin.Context) (int, int, bool) {
	page := 1
	pageSize := 20

	if err := c.ShouldBindQuery(&struct {
		Page     *int `form:"page"`
		PageSize *int `form:"page_size"`
	}{&page, &pageSize}); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid query parameters",
		})
		return 0, 0, false
	}

	if page < 1 || pageSize < 1 || pageSize > 100 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid pagination parameters",
		})
		return 0, 0, false
	}

	return page, pageSize, true
}

func writeMessageServiceError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, services.ErrInvalidMessageID),
		errors.Is(err, services.ErrInvalidSenderID),
		errors.Is(err, services.ErrInvalidRecipientID),
		errors.Is(err, services.ErrInvalidMessageUser):
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
	case errors.Is(err, services.ErrMessageSubjectRequired),
		errors.Is(err, services.ErrMessageBodyRequired),
		errors.Is(err, services.ErrRecipientNotActive):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"success": false, "message": err.Error()})
	case errors.Is(err, services.ErrRecipientNotFound),
		errors.Is(err, repository.ErrMessageNotFound):
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": fallback})
	}
}
