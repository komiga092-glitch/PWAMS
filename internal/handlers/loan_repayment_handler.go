package handlers

import (
	"errors"

	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

type LoanRepaymentHandler struct {
	repaymentService *services.LoanRepaymentService
}

func NewLoanRepaymentHandler(
	repaymentService *services.LoanRepaymentService,
) *LoanRepaymentHandler {
	return &LoanRepaymentHandler{
		repaymentService: repaymentService,
	}
}

func (h *LoanRepaymentHandler) Page(c *gin.Context) {
	query := models.LoanRepaymentListQuery{LoanID: c.Query("loan_id"), Status: c.Query("status")}
	repayments, _, _, _, err := h.repaymentService.List(query)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "base", gin.H{"page_template": "loan_repayments_content", "title": "Loan Repayments", "data": []gin.H{}, "error": "Unable to retrieve repayments"})
		return
	}
	items := make([]gin.H, 0, len(repayments))
	for _, repayment := range repayments {
		items = append(items, gin.H{"ID": repayment.ID, "LoanID": repayment.LoanID, "Amount": repayment.Amount, "DueDate": repayment.DueDate, "PaidAt": repayment.PaidAt})
	}
	c.HTML(http.StatusOK, "base", gin.H{"page_template": "loan_repayments_content", "title": "Loan Repayments", "data": items})
}

// Create creates a repayment schedule entry for a loan.
func (h *LoanRepaymentHandler) Create(c *gin.Context) {
	var request models.CreateLoanRepaymentRequest

	if err := c.ShouldBind(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid repayment request",
			"error":   err.Error(),
		})
		return
	}

	repayment, err := h.repaymentService.Create(request)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidLoanID),
			errors.Is(err, services.ErrInvalidLoanIDFormat):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid loan ID",
			})

		case errors.Is(err, services.ErrInvalidInstallmentNumber),
			errors.Is(err, services.ErrInvalidRepaymentAmount),
			errors.Is(err, services.ErrInvalidRepaymentDueDate):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid repayment details",
			})

		case errors.Is(err, services.ErrInvalidLoanRepayment):
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"message": "Repayment installment already exists",
			})

		case errors.Is(err, repository.ErrLoanNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Loan not found",
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to create repayment",
			})
		}

		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success":   true,
		"message":   "Loan repayment created successfully",
		"repayment": repayment,
	})
}

// GetByID returns one repayment.
func (h *LoanRepaymentHandler) GetByID(c *gin.Context) {
	id := c.Param("id")

	repayment, err := h.repaymentService.GetByID(id)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidLoanRepaymentID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid repayment ID",
			})

		case errors.Is(err, repository.ErrLoanRepaymentNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Repayment not found",
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to retrieve repayment",
			})
		}

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"repayment": repayment,
	})
}

// List returns repayments with optional loan/status filters.
func (h *LoanRepaymentHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	query := models.LoanRepaymentListQuery{
		LoanID:   c.Query("loan_id"),
		Status:   c.Query("status"),
		Page:     page,
		PageSize: pageSize,
	}

	repayments, total, currentPage, currentPageSize, err :=
		h.repaymentService.List(query)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidLoanID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid loan ID",
			})

		case errors.Is(err, services.ErrInvalidRepaymentStatus):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid repayment status",
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to retrieve repayments",
			})
		}

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"repayments": repayments,
			"total":      total,
			"page":       currentPage,
			"page_size":  currentPageSize,
		},
	})
}

// Pay records a payment against a repayment.
func (h *LoanRepaymentHandler) Pay(c *gin.Context) {
	id := c.Param("id")

	var request models.PayLoanRepaymentRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid payment request",
			"error":   err.Error(),
		})
		return
	}

	repayment, err := h.repaymentService.Pay(id, request)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidLoanRepaymentID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid repayment ID",
			})

		case errors.Is(err, repository.ErrLoanRepaymentNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Repayment not found",
			})

		case errors.Is(err, services.ErrInvalidRepaymentAmount):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid payment amount",
			})

		case errors.Is(err, services.ErrRepaymentAlreadyPaid):
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"message": "Repayment is already paid",
			})

		case errors.Is(err, services.ErrRepaymentAmountTooHigh):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Payment amount exceeds remaining amount",
			})

		case errors.Is(err, services.ErrRepaymentCannotBeCancelled):
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"message": "Cancelled repayment cannot be paid",
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to process payment",
			})
		}

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "Repayment payment processed successfully",
		"repayment": repayment,
	})
}

// Cancel cancels an unpaid repayment.
func (h *LoanRepaymentHandler) Cancel(c *gin.Context) {
	id := c.Param("id")

	if err := h.repaymentService.Cancel(id); err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidLoanRepaymentID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid repayment ID",
			})

		case errors.Is(err, repository.ErrLoanRepaymentNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Repayment not found",
			})

		case errors.Is(err, services.ErrRepaymentCannotBeCancelled):
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"message": "Paid repayment cannot be cancelled",
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to create repayment",
				"error":   err.Error(),
			})
		}

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Repayment cancelled successfully",
	})
}
