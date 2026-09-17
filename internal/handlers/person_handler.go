package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/constants"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

type PersonHandler struct {
	personService   *services.PersonService
	auditLogService *services.AuditLogService
}

func NewPersonHandler(
	personService *services.PersonService,
	auditLogServices ...*services.AuditLogService,
) *PersonHandler {
	var auditLogService *services.AuditLogService
	if len(auditLogServices) > 0 {
		auditLogService = auditLogServices[0]
	}

	return &PersonHandler{
		personService:   personService,
		auditLogService: auditLogService,
	}
}

// Create creates a new person.
//
// @Summary Create person
// @Description Create a new beneficiary/person record.
// @Tags Persons
// @Accept json
// @Produce json
// @Param request body models.CreatePersonRequest true "Person information"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{}
// @Failure 422 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /persons [post]
func (h *PersonHandler) Create(c *gin.Context) {
	var request models.CreatePersonRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": constants.ErrInvalidPersonInformation,
		})
		return
	}

	currentUser, ok := getCurrentUser(c)
	if !ok {
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

		case errors.Is(err, services.ErrInvalidPhone):
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"success": false,
				"message": err.Error(),
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": constants.ErrUnableToCreatePerson,
			})
		}

		return
	}

	if err := h.auditLogService.Create(
		currentUser.ID.String(),
		"CREATE",
		"persons",
		person.ID.String(),
		"Person created successfully",
	); err != nil {
		// Audit logging failure must not fail the person creation.
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": constants.ErrPersonCreatedSuccessfully,
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

// List returns persons with search and pagination.
//
// @Summary List persons
// @Description Retrieve persons with search, filtering and pagination.
// @Tags Persons
// @Produce json
// @Param search query string false "Search persons"
// @Param status query string false "Filter by status"
// @Param page query int false "Page number"
// @Param page_size query int false "Number of records per page"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /persons [get]
func (h *PersonHandler) List(c *gin.Context) {
	var query models.PersonListQuery

	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": constants.ErrInvalidQueryParameters,
		})
		return
	}

	currentUser, _ := getCurrentUser(c)
	actor, _ := services.ActorFromUser(currentUser)

	persons, total, page, pageSize, err :=
		h.personService.ListPersons(query, actor)

	if err != nil {
		if errors.Is(err, services.ErrRecordAccessDenied) {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": services.ErrRecordAccessDenied.Error(),
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": constants.ErrUnableToRetrievePersons,
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

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    constants.ErrPersonsRetrievedSuccessfully,
		"data":       items,
		"pagination": buildPagination(total, page, pageSize),
	})
}

// GetByID returns one person by UUID.
//
// @Summary Get person
// @Description Retrieve a single person by UUID.
// @Tags Persons
// @Produce json
// @Param id path string true "Person UUID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /persons/{id} [get]
func (h *PersonHandler) GetByID(c *gin.Context) {
	personID := c.Param("id")

	currentUser, _ := getCurrentUser(c)
	actor, _ := services.ActorFromUser(currentUser)

	person, err := h.personService.GetPersonByID(personID, actor)
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

		case errors.Is(err, services.ErrRecordAccessDenied):
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": services.ErrRecordAccessDenied.Error(),
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": constants.ErrUnableToRetrievePerson,
			})
		}

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": constants.ErrPersonRetrievedSuccessfully,
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

// Update updates an existing person.
//
// @Summary Update person
// @Description Update an existing person record.
// @Tags Persons
// @Accept json
// @Produce json
// @Param id path string true "Person UUID"
// @Param request body models.UpdatePersonRequest true "Updated person information"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{}
// @Failure 422 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /persons/{id} [put]
func (h *PersonHandler) Update(c *gin.Context) {
	personID := c.Param("id")

	var request models.UpdatePersonRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": constants.ErrInvalidPersonInformation,
		})
		return
	}

	currentUser, _ := getCurrentUser(c)
	actor, _ := services.ActorFromUser(currentUser)

	person, err := h.personService.UpdatePerson(
		personID,
		request,
		actor,
	)

	if err != nil {
		writeErrorResponse(c, err, http.StatusInternalServerError, constants.ErrUnableToUpdatePerson,
			errorResponseMapping{err: services.ErrInvalidPersonID, status: http.StatusBadRequest, message: constants.ErrInvalidPersonID},
			errorResponseMapping{err: repository.ErrPersonNotFound, status: http.StatusNotFound, message: constants.ErrPersonNotFound},
			errorResponseMapping{err: services.ErrPersonAlreadyExists, status: http.StatusConflict, message: err.Error()},
			errorResponseMapping{err: services.ErrInvalidDateOfBirth, status: http.StatusUnprocessableEntity, message: err.Error()},
			errorResponseMapping{err: services.ErrInvalidPhone, status: http.StatusUnprocessableEntity, message: err.Error()},
			errorResponseMapping{err: services.ErrInvalidPersonStatus, status: http.StatusUnprocessableEntity, message: err.Error()},
		)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": constants.ErrPersonUpdatedSuccessfully,
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

// UpdateStatus updates the status of a person.
//
// @Summary Update person status
// @Description Change the status of an existing person.
// @Tags Persons
// @Accept json
// @Produce json
// @Param id path string true "Person UUID"
// @Param request body models.UpdatePersonStatusRequest true "Person status"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 422 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /persons/{id}/status [patch]
func (h *PersonHandler) UpdateStatus(c *gin.Context) {
	personID := c.Param("id")

	var request models.UpdatePersonStatusRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": constants.ErrStatusRequired,
		})
		return
	}

	currentUser, _ := getCurrentUser(c)
	actor, _ := services.ActorFromUser(currentUser)

	err := h.personService.UpdatePersonStatus(
		personID,
		request.Status,
		actor,
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
				"message": constants.ErrPersonNotFound,
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": constants.ErrUnableToUpdatePersonStatus,
			})
		}

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": constants.ErrPersonStatusUpdatedSuccessfully,
	})
}

// Delete deletes a person.
//
// @Summary Delete person
// @Description Delete an existing person record.
// @Tags Persons
// @Produce json
// @Param id path string true "Person UUID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /persons/{id} [delete]
func (h *PersonHandler) Delete(c *gin.Context) {
	personID := c.Param("id")

	currentUser, _ := getCurrentUser(c)
	actor, _ := services.ActorFromUser(currentUser)

	err := h.personService.DeletePerson(personID, actor)
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
				"message": constants.ErrUnableToDeletePerson,
			})
		}

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": constants.ErrPersonDeletedSuccessfully,
	})
}

func (h *PersonHandler) Page(c *gin.Context) {
	var query models.PersonListQuery

	if err := c.ShouldBindQuery(&query); err != nil {
		c.HTML(http.StatusBadRequest, "base", PageData(c, gin.H{
			"page_template": "persons_content",
			"title":         "Care Seekers - PWAMS",
			"persons":       []models.Person{},
			"search":        "",
			"status":        "",
			"error":         constants.ErrInvalidQueryParameters,
		}))
		return
	}

	currentUser, _ := getCurrentUser(c)
	actor, _ := services.ActorFromUser(currentUser)

	persons, _, _, _, err := h.personService.ListPersons(query, actor)

	if err != nil {
		c.HTML(http.StatusInternalServerError, "base", PageData(c, gin.H{
			"page_template": "persons_content",
			"title":         "Care Seekers - PWAMS",
			"persons":       []models.Person{},
			"search":        query.Search,
			"status":        query.Status,
			"error":         constants.ErrUnableToRetrievePersons,
		}))
		return
	}

	c.HTML(http.StatusOK, "base", PageData(c, gin.H{
		"page_template": "persons_content",
		"title":         "Care Seekers - PWAMS",
		"persons":       persons,
		"search":        query.Search,
		"status":        query.Status,
	}))
}
func (h *PersonHandler) ViewPage(c *gin.Context) {
	personID := c.Param("id")

	currentUser, _ := getCurrentUser(c)
	actor, _ := services.ActorFromUser(currentUser)

	person, err := h.personService.GetPersonByID(personID, actor)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidPersonID):
			c.HTML(http.StatusBadRequest, "base", PageData(c, gin.H{
				"page_template": "person_view_content",
				"title":         "Invalid Person - PWAMS",
				"error":         constants.ErrInvalidPersonID,
			}))

		case errors.Is(err, repository.ErrPersonNotFound):
			c.HTML(http.StatusNotFound, "base", PageData(c, gin.H{
				"page_template": "person_view_content",
				"title":         "Person Not Found - PWAMS",
				"error":         constants.ErrPersonNotFound,
			}))

		default:
			c.HTML(http.StatusInternalServerError, "base", PageData(c, gin.H{
				"page_template": "person_view_content",
				"title":         "Person - PWAMS",
				"error":         constants.ErrUnableToRetrievePerson,
			}))
		}

		return
	}

	c.HTML(http.StatusOK, "base", PageData(c, gin.H{
		"page_template": "person_view_content",
		"title":         "Care Seeker Details - PWAMS",
		"person":        person,
	}))
}
func (h *PersonHandler) EditPage(c *gin.Context) {
	personID := c.Param("id")

	currentUser, _ := getCurrentUser(c)
	actor, _ := services.ActorFromUser(currentUser)

	person, err := h.personService.GetPersonByID(personID, actor)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidPersonID):
			c.HTML(http.StatusBadRequest, "base", PageData(c, gin.H{
				"page_template": "person_edit_content",
				"title":         "Invalid Person - PWAMS",
				"error":         constants.ErrInvalidPersonID,
			}))

		case errors.Is(err, repository.ErrPersonNotFound):
			c.HTML(http.StatusNotFound, "base", PageData(c, gin.H{
				"page_template": "person_edit_content",
				"title":         "Person Not Found - PWAMS",
				"error":         constants.ErrPersonNotFound,
			}))

		default:
			c.HTML(http.StatusInternalServerError, "base", PageData(c, gin.H{
				"page_template": "person_edit_content",
				"title":         "Edit Person - PWAMS",
				"error":         constants.ErrUnableToRetrievePerson,
			}))
		}

		return
	}

	c.HTML(http.StatusOK, "base", PageData(c, gin.H{
		"page_template": "person_edit_content",
		"title":         "Edit Care Seeker - PWAMS",
		"person":        person,
	}))
}
