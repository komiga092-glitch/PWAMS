package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

type LoanHandler struct {
	loanService     *services.LoanService
	auditLogService *services.AuditLogService
}

func NewLoanHandler(
	loanService *services.LoanService,
	auditLogService *services.AuditLogService,
) *LoanHandler {
	return &LoanHandler{
		loanService:     loanService,
		auditLogService: auditLogService,
	}
}

func (h *LoanHandler) Page(c *gin.Context) {
	var query models.LoanListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.HTML(http.StatusBadRequest, "base", PageData(c, gin.H{"page_template": "loans_content", "title": "Loans", "data": []gin.H{}, "error": "Invalid query parameters"}))
		return
	}

	currentUser, _ := getCurrentUser(c)
	actor, _ := services.ActorFromUser(currentUser)

	loans, _, _, _, err := h.loanService.ListLoans(query, actor)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "base", PageData(c, gin.H{"page_template": "loans_content", "title": "Loans", "data": []gin.H{}, "error": "Unable to retrieve loans"}))
		return
	}
	items := make([]gin.H, 0, len(loans))
	for _, loan := range loans {
		items = append(items, gin.H{"ID": loan.ID, "PersonID": loan.PersonID, "PersonName": loan.Person.FullName, "LoanAmount": loan.LoanAmount, "InterestRate": loan.InterestRate, "DurationMonths": loan.DurationMonths, "InstallmentAmount": loan.InstallmentAmount, "Status": loan.Status})
	}
	c.HTML(http.StatusOK, "base", PageData(c, gin.H{"page_template": "loans_content", "title": "Loans", "data": items, "search": "", "status": query.Status}))
}

func (h *LoanHandler) Create(c *gin.Context) {
	var request models.CreateLoanRequest

	if err := c.ShouldBind(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid loan request",
		})
		return
	}

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

	var loan *models.Loan

	// The loan row and its mandatory audit entry are committed together or
	// not at all. An unavailable audit dependency fails closed before the
	// business write is attempted (no panic, no orphaned record).
	err := h.auditLogService.Transaction(func(tx *gorm.DB) error {
		created, createErr := h.loanService.WithTx(tx).CreateLoan(
			request,
			currentUser.ID,
		)
		if createErr != nil {
			return createErr
		}

		loan = created

		return h.auditLogService.Audit(
			tx,
			currentUser.ID.String(),
			"CREATE",
			"loans",
			created.ID.String(),
			"Loan created successfully",
		)
	})
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidLoanAmount):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Loan amount must be greater than zero",
			})

		case errors.Is(err, services.ErrInvalidInterestRate):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Interest rate cannot be negative",
			})

		case errors.Is(err, services.ErrInvalidLoanDuration):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Loan duration must be greater than zero",
			})

		case errors.Is(err, services.ErrInvalidPersonID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid person ID",
			})

		case errors.Is(err, services.ErrLoanAmountOutOfRange),
			errors.Is(err, services.ErrLoanDurationOutOfRange):
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"success": false,
				"message": err.Error(),
			})

		case errors.Is(err, services.ErrPersonNotActive):
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"success": false,
				"message": err.Error(),
			})

		case errors.Is(err, services.ErrPersonHasActiveLoan):
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"message": err.Error(),
			})

		case errors.Is(err, repository.ErrPersonNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Person not found",
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to create loan",
			})
		}

		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Loan created successfully",
		"loan":    loan,
	})
}

func (h *LoanHandler) GetByID(c *gin.Context) {
	id := c.Param("id")

	currentUser, _ := getCurrentUser(c)
	actor, _ := services.ActorFromUser(currentUser)

	loan, err := h.loanService.GetLoanByID(id, actor)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidLoanID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid loan ID",
			})

		case errors.Is(err, repository.ErrLoanNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Loan not found",
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to retrieve loan",
			})
		}

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"loan":    loan,
	})
}

func (h *LoanHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	query := models.LoanListQuery{
		Search:   c.Query("search"),
		PersonID: c.Query("person_id"),
		Status:   c.Query("status"),
		Page:     page,
		PageSize: pageSize,
	}

	currentUser, _ := getCurrentUser(c)
	actor, _ := services.ActorFromUser(currentUser)

	loans, total, currentPage, currentPageSize, err :=
		h.loanService.ListLoans(query, actor)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Unable to retrieve loans",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"loans":     loans,
			"total":     total,
			"page":      currentPage,
			"page_size": currentPageSize,
		},
	})
}

func (h *LoanHandler) Review(c *gin.Context) {
	id := c.Param("id")

	var request models.ReviewLoanRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid review request",
		})
		return
	}

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

	if currentUser.ID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Invalid user",
		})
		return
	}

	actor, _ := services.ActorFromUser(currentUser)

	loan, err := h.loanService.ReviewLoan(
		id,
		request,
		currentUser.ID,
		actor,
	)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidLoanID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid loan ID",
			})

		case errors.Is(err, repository.ErrLoanNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Loan not found",
			})

		case errors.Is(err, services.ErrInvalidLoanStatus),
			errors.Is(err, services.ErrInvalidLoanStatusTransition):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid loan status transition",
			})

		case errors.Is(err, services.ErrCannotReviewOwnSubmission):
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": err.Error(),
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to review loan",
			})
		}

		return
	}

	if currentUser, ok := getCurrentUser(c); ok {
		if auditErr := h.auditLogService.Create(
			currentUser.ID.String(),
			"STATUS_CHANGE",
			"loans",
			id,
			"Loan reviewed with status "+request.Status,
		); auditErr != nil {
			// Audit logging failure must not fail the loan review.
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Loan status updated successfully",
		"loan":    loan,
	})
}

// Update applies an edit to a loan that is still awaiting review. It is the
// HTTP surface of models.UpdateLoanRequest / LoanService.UpdateLoan (QA
// LON-009 / LON-010 / LON-011).
func (h *LoanHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var request models.UpdateLoanRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid loan update request",
		})
		return
	}

	currentUser, ok := getCurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Authentication required",
		})
		return
	}

	actor, _ := services.ActorFromUser(currentUser)

	loan, err := h.loanService.UpdateLoan(id, request, actor)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidLoanID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid loan ID",
			})

		case errors.Is(err, repository.ErrLoanNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Loan not found",
			})

		case errors.Is(err, services.ErrRecordAccessDenied):
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": err.Error(),
			})

		case errors.Is(err, services.ErrLoanCannotBeEdited):
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"message": err.Error(),
			})

		case errors.Is(err, services.ErrInvalidLoanAmount):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Loan amount must be greater than zero",
			})

		case errors.Is(err, services.ErrInvalidInterestRate):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Interest rate cannot be negative",
			})

		case errors.Is(err, services.ErrInvalidLoanDuration):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Loan duration must be greater than zero",
			})

		case errors.Is(err, services.ErrLoanAmountOutOfRange),
			errors.Is(err, services.ErrLoanDurationOutOfRange):
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"success": false,
				"message": err.Error(),
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to update loan",
			})
		}

		return
	}

	if currentUser, ok := getCurrentUser(c); ok {
		if auditErr := h.auditLogService.Create(
			currentUser.ID.String(),
			"UPDATE",
			"loans",
			id,
			"Loan updated successfully",
		); auditErr != nil {
			// Audit logging failure must not fail the loan update.
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Loan updated successfully",
		"loan":    loan,
	})
}
