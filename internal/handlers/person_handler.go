package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/services"
)

type PersonHandler struct {
	personService *services.PersonService
}

func NewPersonHandler(
	personService *services.PersonService,
) *PersonHandler {
	return &PersonHandler{
		personService: personService,
	}
}

func (h *PersonHandler) Create(c *gin.Context) {
	var request models.CreatePersonRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid person information",
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

	person, err := h.personService.CreatePerson(
		request,
		currentUser.ID,
	)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrPersonAlreadyExists):
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"message": err.Error(),
			})

		case errors.Is(err, services.ErrInvalidDateOfBirth):
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"success": false,
				"message": err.Error(),
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to create person",
			})
		}

		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Person registered successfully",
		"person": gin.H{
			"id":             person.ID,
			"full_name":      person.FullName,
			"nic_passport":   person.NICPassport,
			"date_of_birth":  person.DateOfBirth,
			"gender":         person.Gender,
			"phone":          person.Phone,
			"email":          person.Email,
			"address":        person.Address,
			"occupation":     person.Occupation,
			"monthly_income": person.MonthlyIncome,
			"status":         person.Status,
			"created_by_id":  person.CreatedByID,
		},
	})
}
