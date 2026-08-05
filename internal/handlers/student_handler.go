package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

type StudentHandler struct {
	studentService *services.StudentService
}

func NewStudentHandler(
	studentService *services.StudentService,
) *StudentHandler {
	return &StudentHandler{
		studentService: studentService,
	}
}

func (h *StudentHandler) Create(c *gin.Context) {
	var request models.CreateStudentRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid student information",
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

	student, err := h.studentService.CreateStudent(
		request,
		currentUser.ID,
	)

	if err != nil {
		switch {
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

		case errors.Is(err, services.ErrStudentAlreadyExists):
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"message": err.Error(),
			})

		case errors.Is(err, services.ErrInvalidStudentDateOfBirth):
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"success": false,
				"message": err.Error(),
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to create student",
			})
		}

		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Student registered successfully",
		"student": gin.H{
			"id":             student.ID,
			"person_id":      student.PersonID,
			"full_name":      student.FullName,
			"school_name":    student.SchoolName,
			"grade":          student.Grade,
			"student_code":   student.StudentCode,
			"date_of_birth":  student.DateOfBirth,
			"gender":         student.Gender,
			"guardian_name":  student.GuardianName,
			"guardian_phone": student.GuardianPhone,
			"academic_year":  student.AcademicYear,
			"remarks":        student.Remarks,
			"status":         student.Status,
			"created_by_id":  student.CreatedByID,
		},
	})
}
