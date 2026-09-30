package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/komiga092-glitch/pwams/internal/constants"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

type StudentHandler struct {
	studentService  *services.StudentService
	auditLogService *services.AuditLogService
}

func NewStudentHandler(
	studentService *services.StudentService,
	auditLogService *services.AuditLogService,
) *StudentHandler {
	return &StudentHandler{
		studentService:  studentService,
		auditLogService: auditLogService,
	}
}

func (h *StudentHandler) Create(c *gin.Context) {
	var request models.CreateStudentRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": constants.ErrInvalidStudentInformation,
		})
		return
	}

	currentUser, ok := getCurrentUser(c)
	if !ok {
		return
	}

	var student *models.Student

	// The student row and its mandatory audit entry are committed together
	// or not at all. An unavailable audit dependency fails closed before the
	// business write is attempted (no panic, no orphaned record).
	err := h.auditLogService.Transaction(func(tx *gorm.DB) error {
		created, createErr := h.studentService.WithTx(tx).CreateStudent(
			request,
			currentUser.ID,
		)
		if createErr != nil {
			return createErr
		}

		student = created

		return h.auditLogService.Audit(
			tx,
			currentUser.ID.String(),
			"CREATE",
			"students",
			created.ID.String(),
			"Student created successfully",
		)
	})

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

		case errors.Is(err, services.ErrInvalidPhone):
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"success": false,
				"message": err.Error(),
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": constants.ErrUnableToCreateStudent,
			})
		}

		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": constants.ErrStudentCreatedSuccessfully,
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
			"message": constants.ErrInvalidQueryParameters,
		})
		return
	}

	currentUser, _ := getCurrentUser(c)
	actor, _ := services.ActorFromUser(currentUser)

	students, total, page, pageSize, err :=
		h.studentService.ListStudents(query, actor)

	if err != nil {
		if errors.Is(err, services.ErrRecordAccessDenied) {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": services.ErrRecordAccessDenied.Error(),
			})
			return
		}

		if errors.Is(err, services.ErrInvalidPersonID) {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": constants.ErrInvalidPersonID,
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": constants.ErrUnableToRetrieveStudents,
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

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    constants.ErrStudentsRetrievedSuccessfully,
		"data":       items,
		"pagination": buildPagination(total, page, pageSize),
	})
}

func (h *StudentHandler) GetByID(c *gin.Context) {
	studentID := c.Param("id")

	currentUser, _ := getCurrentUser(c)
	actor, _ := services.ActorFromUser(currentUser)

	student, err := h.studentService.GetStudentByID(studentID, actor)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidStudentID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": constants.ErrInvalidStudentID,
			})

		case errors.Is(err, repository.ErrStudentNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": constants.ErrStudentNotFound,
			})

		case errors.Is(err, services.ErrRecordAccessDenied):
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": services.ErrRecordAccessDenied.Error(),
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": constants.ErrUnableToRetrieveStudent,
			})
		}

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": constants.ErrStudentRetrievedSuccessfully,
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

func (h *StudentHandler) ViewPage(c *gin.Context) {
	studentID := c.Param("id")

	currentUser, _ := getCurrentUser(c)
	actor, _ := services.ActorFromUser(currentUser)

	student, err := h.studentService.GetStudentByID(studentID, actor)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidStudentID):
			c.HTML(http.StatusBadRequest, "base", PageData(c, gin.H{
				"page_template": "student_view",
				"title":         "Invalid Student - PWAMS",
				"error":         constants.ErrInvalidStudentID,
			}))

		case errors.Is(err, repository.ErrStudentNotFound):
			c.HTML(http.StatusNotFound, "base", PageData(c, gin.H{
				"page_template": "student_view",
				"title":         "Student Not Found - PWAMS",
				"error":         constants.ErrStudentNotFound,
			}))

		default:
			c.HTML(http.StatusInternalServerError, "base", PageData(c, gin.H{
				"page_template": "student_view",
				"title":         "Student - PWAMS",
				"error":         constants.ErrUnableToRetrieveStudent,
			}))
		}

		return
	}

	c.HTML(http.StatusOK, "base", PageData(c, gin.H{
		"page_template": "student_view",
		"title":         "Student Details - PWAMS",
		"student":       student,
	}))
}

func (h *StudentHandler) EditPage(c *gin.Context) {
	studentID := c.Param("id")

	currentUser, _ := getCurrentUser(c)
	actor, _ := services.ActorFromUser(currentUser)

	student, err := h.studentService.GetStudentByID(studentID, actor)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidStudentID):
			c.HTML(http.StatusBadRequest, "base", PageData(c, gin.H{
				"page_template": "student_edit",
				"title":         "Invalid Student - PWAMS",
				"error":         constants.ErrInvalidStudentID,
			}))

		case errors.Is(err, repository.ErrStudentNotFound):
			c.HTML(http.StatusNotFound, "base", PageData(c, gin.H{
				"page_template": "student_edit",
				"title":         "Student Not Found - PWAMS",
				"error":         constants.ErrStudentNotFound,
			}))

		default:
			c.HTML(http.StatusInternalServerError, "base", PageData(c, gin.H{
				"page_template": "student_edit",
				"title":         "Edit Student - PWAMS",
				"error":         constants.ErrUnableToRetrieveStudent,
			}))
		}

		return
	}

	c.HTML(http.StatusOK, "base", PageData(c, gin.H{
		"page_template": "student_edit",
		"title":         "Edit Student - PWAMS",
		"student":       student,
	}))
}

func (h *StudentHandler) Update(c *gin.Context) {
	studentID := c.Param("id")

	var request models.UpdateStudentRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": constants.ErrInvalidStudentInformation,
		})
		return
	}

	currentUser, _ := getCurrentUser(c)
	actor, _ := services.ActorFromUser(currentUser)

	student, err := h.studentService.UpdateStudent(
		studentID,
		request,
		actor,
	)

	if err != nil {
		writeErrorResponse(c, err, http.StatusInternalServerError, constants.ErrUnableToUpdateStudent,
			errorResponseMapping{err: services.ErrInvalidStudentID, status: http.StatusBadRequest, message: constants.ErrInvalidStudentID},
			errorResponseMapping{err: services.ErrInvalidPersonID, status: http.StatusBadRequest, message: constants.ErrInvalidPersonID},
			errorResponseMapping{err: repository.ErrStudentNotFound, status: http.StatusNotFound, message: constants.ErrStudentNotFound},
			errorResponseMapping{err: repository.ErrPersonNotFound, status: http.StatusNotFound, message: constants.ErrPersonNotFound},
			errorResponseMapping{err: services.ErrStudentAlreadyExists, status: http.StatusConflict, message: constants.ErrStudentAlreadyExists},
			errorResponseMapping{err: services.ErrInvalidStudentDateOfBirth, status: http.StatusUnprocessableEntity, message: err.Error()},
			errorResponseMapping{err: services.ErrInvalidPhone, status: http.StatusUnprocessableEntity, message: err.Error()},
			errorResponseMapping{err: services.ErrInvalidStudentStatus, status: http.StatusUnprocessableEntity, message: err.Error()},
		)
		return
	}

	if auditErr := h.auditLogService.Create(
		currentUser.ID.String(),
		"UPDATE",
		"students",
		student.ID.String(),
		"Student updated successfully",
	); auditErr != nil {
		// Audit logging failure must not fail the student update.
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": constants.ErrStudentUpdatedSuccessfully,
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
			"message": constants.ErrStatusRequired,
		})
		return
	}

	currentUser, _ := getCurrentUser(c)
	actor, _ := services.ActorFromUser(currentUser)

	err := h.studentService.UpdateStudentStatus(
		studentID,
		request.Status,
		actor,
	)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidStudentID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": constants.ErrInvalidStudentID,
			})

		case errors.Is(err, services.ErrInvalidStudentStatus):
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"success": false,
				"message": constants.ErrInvalidStudentStatus,
			})

		case errors.Is(err, repository.ErrStudentNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": constants.ErrStudentNotFound,
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": constants.ErrUnableToUpdateStudentStatus,
			})
		}

		return
	}

	if auditErr := h.auditLogService.Create(
		currentUser.ID.String(),
		"STATUS_CHANGE",
		"students",
		studentID,
		"Student status changed to "+request.Status,
	); auditErr != nil {
		// Audit logging failure must not fail the student status update.
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": constants.ErrStudentStatusUpdatedSuccessfully,
	})
}

func (h *StudentHandler) Delete(c *gin.Context) {
	studentID := c.Param("id")

	currentUser, _ := getCurrentUser(c)
	actor, _ := services.ActorFromUser(currentUser)

	err := h.studentService.DeleteStudent(studentID, actor)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidStudentID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": constants.ErrInvalidStudentID,
			})

		case errors.Is(err, repository.ErrStudentNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": constants.ErrStudentNotFound,
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": constants.ErrUnableToDeleteStudent,
			})
		}

		return
	}

	if auditErr := h.auditLogService.Create(
		currentUser.ID.String(),
		"DELETE",
		"students",
		studentID,
		"Student deleted successfully",
	); auditErr != nil {
		// Audit logging failure must not fail the student deletion.
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": constants.ErrStudentDeletedSuccessfully,
	})
}

func (h *StudentHandler) Page(c *gin.Context) {
	var query models.StudentListQuery

	if err := c.ShouldBindQuery(&query); err != nil {
		c.HTML(http.StatusBadRequest, "base", PageData(c, gin.H{
			"page_template": "students_content",
			"title":         "Students - PWAMS",
			"students":      []interface{}{},
			"search":        "",
			"status":        "",
			"error":         constants.ErrInvalidQueryParameters,
		}))
		return
	}

	currentUser, _ := getCurrentUser(c)
	actor, _ := services.ActorFromUser(currentUser)

	students, _, _, _, err := h.studentService.ListStudents(query, actor)

	if err != nil {
		c.HTML(http.StatusInternalServerError, "base", PageData(c, gin.H{
			"page_template": "students_content",
			"title":         "Students - PWAMS",
			"students":      []interface{}{},
			"search":        query.Search,
			"status":        query.Status,
			"error":         constants.ErrUnableToRetrieveStudents,
		}))
		return
	}

	items := make([]gin.H, 0, len(students))

	for _, student := range students {
		items = append(items, gin.H{
			"ID":          student.ID,
			"Name":        student.FullName,
			"StudentID":   student.StudentCode,
			"Phone":       student.GuardianPhone,
			"Institution": student.SchoolName,
			"Status":      student.Status,
		})
	}

	c.HTML(http.StatusOK, "base", PageData(c, gin.H{
		"page_template": "students_content",
		"title":         "Students - PWAMS",
		"students":      items,
		"search":        query.Search,
		"status":        query.Status,
	}))
}
