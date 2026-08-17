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
	loanService *services.LoanService
}

func NewLoanHandler(
	loanService *services.LoanService,
) *LoanHandler {
	return &LoanHandler{
		loanService: loanService,
	}
}

func (h *LoanHandler) Create(c *gin.Context) {
	var request models.CreateLoanRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid loan request",
			"error":   err.Error(),
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

	loan, err := h.loanService.GetLoanByID(id)
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

	loans, total, currentPage, currentPageSize, err :=
		h.loanService.ListLoans(query)

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
			"error":   err.Error(),
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

	loan, err := h.loanService.ReviewLoan(
		id,
		request,
		currentUser.ID,
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
