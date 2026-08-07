package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
	"github.com/komiga092-glitch/pwams/internal/constants"
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
			"message": constants.ErrAuthenticationRequired,
		})
		return
	}

	currentUser, ok := currentUserValue.(*models.User)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": constants.ErrInvalidAuthContext,
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

func (h *PersonHandler) List(c *gin.Context) {
	var query models.PersonListQuery

	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid query parameters",
		})
		return
	}

	persons, total, page, pageSize, err :=
		h.personService.ListPersons(query)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Unable to retrieve persons",
		})
		return
	}

	items := make([]gin.H, 0, len(persons))

	for _, person := range persons {
		items = append(items, gin.H{
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
			"created_by":     person.CreatedBy.Username,
			"created_at":     person.CreatedAt,
		})
	}

	totalPages := 0
	if total > 0 {
		totalPages = int(
			(total + int64(pageSize) - 1) / int64(pageSize),
		)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Persons retrieved successfully",
		"data":    items,
		"pagination": gin.H{
			"page":        page,
			"page_size":   pageSize,
			"total_items": total,
			"total_pages": totalPages,
		},
	})
}

func (h *PersonHandler) GetByID(c *gin.Context) {
	personID := c.Param("id")

	person, err := h.personService.GetPersonByID(personID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidPersonID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": constants.ErrInvalidPersonID,
			})

		case errors.Is(err, repository.ErrPersonNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": constants.ErrPersonNotFound,
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to retrieve person",
			})
		}

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Person retrieved successfully",
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
			"created_by":     person.CreatedBy.Username,
			"created_at":     person.CreatedAt,
			"updated_at":     person.UpdatedAt,
		},
	})
}

func (h *PersonHandler) Update(c *gin.Context) {
	personID := c.Param("id")

	var request models.UpdatePersonRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid person information",
			"error":   err.Error(),
		})
		return
	}

	person, err := h.personService.UpdatePerson(
		personID,
		request,
	)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidPersonID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": constants.ErrInvalidPersonID,
			})

		case errors.Is(err, repository.ErrPersonNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": constants.ErrPersonNotFound,
			})

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

		case errors.Is(err, services.ErrInvalidPersonStatus):
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"success": false,
				"message": err.Error(),
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to update person",
			})
		}

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Person updated successfully",
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
			"updated_at":     person.UpdatedAt,
		},
	})
}

func (h *PersonHandler) UpdateStatus(c *gin.Context) {
	personID := c.Param("id")

	var request models.UpdatePersonStatusRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Status is required",
		})
		return
	}

	err := h.personService.UpdatePersonStatus(
		personID,
		request.Status,
	)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidPersonID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": constants.ErrInvalidPersonID,
			})

		case errors.Is(err, services.ErrInvalidPersonStatus):
			c.JSON(http.StatusUnprocessableEntity, gin.H{
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
				"message": "Unable to update person status",
			})
		}

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Person status updated successfully",
	})
}
func (h *PersonHandler) Delete(c *gin.Context) {
	personID := c.Param("id")

	err := h.personService.DeletePerson(personID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidPersonID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": constants.ErrInvalidPersonID,
			})

		case errors.Is(err, repository.ErrPersonNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": constants.ErrPersonNotFound,
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to delete person",
			})
		}

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Person deleted successfully",
	})
}
