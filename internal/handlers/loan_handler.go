package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

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
	auditLogServices ...*services.AuditLogService,
) *LoanHandler {
	var auditLogService *services.AuditLogService
	if len(auditLogServices) > 0 {
		auditLogService = auditLogServices[0]
	}

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
		items = append(items, gin.H{"ID": loan.ID, "PersonID": loan.PersonID, "LoanAmount": loan.LoanAmount, "InterestRate": loan.InterestRate, "DurationMonths": loan.DurationMonths, "InstallmentAmount": loan.InstallmentAmount, "Status": loan.Status})
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

	loan, err := h.loanService.CreateLoan(
		request,
		currentUser.ID,
	)
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

	if err := h.auditLogService.Create(
		currentUser.ID.String(),
		"CREATE",
		"loans",
		loan.ID.String(),
		"Loan created successfully",
	); err != nil {
		// Audit logging failure must not fail the loan creation.
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

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to review loan",
			})
		}

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Loan status updated successfully",
		"loan":    loan,
	})
}
