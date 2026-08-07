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

	student, err := h.studentService.CreateStudent(
		request,
		currentUser.ID,
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

func (h *StudentHandler) List(c *gin.Context) {
	var query models.StudentListQuery

	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid query parameters",
		})
		return
	}

	students, total, page, pageSize, err :=
		h.studentService.ListStudents(query)

	if err != nil {
		if errors.Is(err, services.ErrInvalidPersonID) {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": constants.ErrInvalidPersonID,
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Unable to retrieve students",
		})
		return
	}

	items := make([]gin.H, 0, len(students))

	for _, student := range students {
		items = append(items, gin.H{
			"id":             student.ID,
			"person_id":      student.PersonID,
			"person_name":    student.Person.FullName,
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
			"created_by":     student.CreatedBy.Username,
			"created_at":     student.CreatedAt,
			"updated_at":     student.UpdatedAt,
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
		"message": "Students retrieved successfully",
		"data":    items,
		"pagination": gin.H{
			"page":        page,
			"page_size":   pageSize,
			"total_items": total,
			"total_pages": totalPages,
		},
	})
}

func (h *StudentHandler) GetByID(c *gin.Context) {
	studentID := c.Param("id")

	student, err := h.studentService.GetStudentByID(studentID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidStudentID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid student ID",
			})

		case errors.Is(err, repository.ErrStudentNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Student not found",
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to retrieve student",
			})
		}

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Student retrieved successfully",
		"student": gin.H{
			"id":             student.ID,
			"person_id":      student.PersonID,
			"person_name":    student.Person.FullName,
			"person_nic":     student.Person.NICPassport,
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
			"created_by":     student.CreatedBy.Username,
			"created_at":     student.CreatedAt,
			"updated_at":     student.UpdatedAt,
		},
	})
}

func (h *StudentHandler) Update(c *gin.Context) {
	studentID := c.Param("id")

	var request models.UpdateStudentRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid student information",
			"error":   err.Error(),
		})
		return
	}

	student, err := h.studentService.UpdateStudent(
		studentID,
		request,
	)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidStudentID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid student ID",
			})

		case errors.Is(err, services.ErrInvalidPersonID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": constants.ErrInvalidPersonID,
			})

		case errors.Is(err, repository.ErrStudentNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Student not found",
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

		case errors.Is(err, services.ErrInvalidStudentStatus):
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"success": false,
				"message": err.Error(),
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to update student",
			})
		}

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Student updated successfully",
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
			"updated_at":     student.UpdatedAt,
		},
	})
}

func (h *StudentHandler) UpdateStatus(c *gin.Context) {
	studentID := c.Param("id")

	var request models.UpdateStudentStatusRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Status is required",
		})
		return
	}

	err := h.studentService.UpdateStudentStatus(
		studentID,
		request.Status,
	)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidStudentID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid student ID",
			})

		case errors.Is(err, services.ErrInvalidStudentStatus):
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"success": false,
				"message": err.Error(),
			})

		case errors.Is(err, repository.ErrStudentNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Student not found",
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to update student status",
			})
		}

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Student status updated successfully",
	})
}

func (h *StudentHandler) Delete(c *gin.Context) {
	studentID := c.Param("id")

	err := h.studentService.DeleteStudent(studentID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidStudentID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid student ID",
			})

		case errors.Is(err, repository.ErrStudentNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Student not found",
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to delete student",
			})
		}

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Student deleted successfully",
	})
}
