package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/services"
)

type AccountActivationHandler struct {
	activationService *services.AccountActivationService
}

func NewAccountActivationHandler(
	activationService *services.AccountActivationService,
) *AccountActivationHandler {
	return &AccountActivationHandler{
		activationService: activationService,
	}
}

func (h *AccountActivationHandler) RequestActivation(c *gin.Context) {
	var request models.RequestActivationRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "valid email is required",
		})
		return
	}

	if err := h.activationService.RequestActivation(request.Email); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "account activation OTP sent successfully",
	})
}

func (h *AccountActivationHandler) VerifyOTP(c *gin.Context) {
	var request models.VerifyActivationOTPRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "email and valid 6-digit OTP are required",
		})
		return
	}

	if err := h.activationService.VerifyActivationOTP(
		request.Email,
		request.OTP,
	); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "activation OTP verified successfully",
	})
}

func (h *AccountActivationHandler) Reactivate(c *gin.Context) {
	var request models.ReactivateAccountRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "email and valid 6-digit OTP are required",
		})
		return
	}

	if err := h.activationService.ReactivateAccount(
		request.Email,
		request.OTP,
	); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "account reactivated successfully",
	})
}
