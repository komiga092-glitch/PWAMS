package handlers

import (
	"errors"
	"fmt"

	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

type LoanRepaymentHandler struct {
	repaymentService *services.LoanRepaymentService
	auditLogService  *services.AuditLogService
}

func NewLoanRepaymentHandler(
	repaymentService *services.LoanRepaymentService,
	auditLogService *services.AuditLogService,
) *LoanRepaymentHandler {
	return &LoanRepaymentHandler{
		repaymentService: repaymentService,
		auditLogService:  auditLogService,
	}
}

func (h *LoanRepaymentHandler) Page(c *gin.Context) {
	query := models.LoanRepaymentListQuery{LoanID: c.Query("loan_id"), Status: c.Query("status")}

	currentUser, _ := getCurrentUser(c)
	actor, _ := services.ActorFromUser(currentUser)

	repayments, _, _, _, err := h.repaymentService.List(query, actor)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "base", PageData(c, gin.H{"page_template": "loan_repayments_content", "title": "Loan Repayments", "data": []gin.H{}, "error": "Unable to retrieve repayments"}))
		return
	}
	items := make([]gin.H, 0, len(repayments))
	for _, repayment := range repayments {
		items = append(items, gin.H{"ID": repayment.ID, "LoanID": repayment.LoanID, "Amount": repayment.Amount, "DueDate": repayment.DueDate, "PaidAt": repayment.PaidAt})
	}
	c.HTML(http.StatusOK, "base", PageData(c, gin.H{"page_template": "loan_repayments_content", "title": "Loan Repayments", "data": items}))
}

// Create creates a repayment schedule entry for a loan.
func (h *LoanRepaymentHandler) Create(c *gin.Context) {
	var request models.CreateLoanRepaymentRequest

	if err := c.ShouldBind(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid repayment request",
		})
		return
	}

	currentUser, _ := getCurrentUser(c)
	actor, _ := services.ActorFromUser(currentUser)

	var repayment *models.LoanRepayment

	// The repayment and its mandatory audit entry are committed together or
	// not at all (FR-16 / NFR-09): a repayment can never exist without its
	// audit trail, and an audit failure rolls the create back.
	err := h.auditLogService.Transaction(func(tx *gorm.DB) error {
		created, createErr := h.repaymentService.WithTx(tx).Create(request, actor)
		if createErr != nil {
			return createErr
		}

		repayment = created

		userID := ""
		if currentUser != nil {
			userID = currentUser.ID.String()
		}

		return h.auditLogService.Audit(
			tx,
			userID,
			"CREATE",
			"loan_repayments",
			created.ID.String(),
			"Loan repayment created successfully",
		)
	})
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

	currentUser, _ := getCurrentUser(c)
	actor, _ := services.ActorFromUser(currentUser)

	repayment, err := h.repaymentService.GetByID(id, actor)
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

	currentUser, _ := getCurrentUser(c)
	actor, _ := services.ActorFromUser(currentUser)

	repayments, total, currentPage, currentPageSize, err :=
		h.repaymentService.List(query, actor)

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
		})
		return
	}

	currentUser, _ := getCurrentUser(c)
	actor, _ := services.ActorFromUser(currentUser)

	var repayment *models.LoanRepayment

	// The payment and its mandatory audit entry are committed together or
	// not at all (FR-16 / NFR-09). A payment must never be recorded without
	// its audit trail, and the response is only sent after both succeed.
	err := h.auditLogService.Transaction(func(tx *gorm.DB) error {
		paid, payErr := h.repaymentService.WithTx(tx).Pay(id, request, actor)
		if payErr != nil {
			return payErr
		}

		repayment = paid

		details := fmt.Sprintf(
			"paid_amount=%s status=%s installment=%d",
			paid.PaidAmount.String(),
			paid.Status,
			paid.InstallmentNumber,
		)

		userID := ""
		if currentUser != nil {
			userID = currentUser.ID.String()
		}

		return h.auditLogService.Audit(
			tx,
			userID,
			"LOAN_PAYMENT",
			"loan_repayments",
			id,
			details,
		)
	})
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

		case errors.Is(err, services.ErrLoanNotPayable):
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"message": "Payments are only allowed on active loans",
			})

		case errors.Is(err, services.ErrRepaymentModified):
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"message": "This repayment was updated by another user. Please reload and retry.",
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

	currentUser, _ := getCurrentUser(c)
	actor, _ := services.ActorFromUser(currentUser)

	// The cancellation and its mandatory audit entry are committed together
	// or not at all (FR-16 / NFR-09).
	err := h.auditLogService.Transaction(func(tx *gorm.DB) error {
		if cancelErr := h.repaymentService.WithTx(tx).Cancel(id, actor); cancelErr != nil {
			return cancelErr
		}

		userID := ""
		if currentUser != nil {
			userID = currentUser.ID.String()
		}

		return h.auditLogService.Audit(
			tx,
			userID,
			"CANCEL",
			"loan_repayments",
			id,
			"Repayment cancelled successfully",
		)
	})
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

		case errors.Is(err, services.ErrRepaymentCannotBeCancelled):
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"message": "Paid repayment cannot be cancelled",
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to create repayment",
			})
		}

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Repayment cancelled successfully",
	})
}
