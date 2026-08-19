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

type UserHandler struct {
	userService     *services.UserService
	auditLogService *services.AuditLogService
}

func NewUserHandler(
	userService *services.UserService,
	auditLogService *services.AuditLogService,
) *UserHandler {
	return &UserHandler{
		userService:     userService,
		auditLogService: auditLogService,
	}
}

// Page renders the users management page.
func (h *UserHandler) Page(c *gin.Context) {
	c.HTML(http.StatusOK, "base", gin.H{
		"title":         "Users",
		"page_template": "users_content",
	})
}

// Create creates a new system user.
//
// @Summary Create user
// @Description Create a new system user.
// @Tags Users
// @Accept json
// @Produce json
// @Param request body models.CreateUserRequest true "User information"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{}
// @Failure 422 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /users [post]
func (h *UserHandler) Create(c *gin.Context) {
	var request models.CreateUserRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid user information",
			"error":   err.Error(),
		})
		return
	}

	currentUser, ok := h.getCurrentUser(c)
	if !ok {
		return
	}

	user, err := h.userService.CreateUser(request)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrUserAlreadyExists):
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"message": err.Error(),
			})

		case errors.Is(err, services.ErrInvalidRole):
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"success": false,
				"message": err.Error(),
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to create user",
			})
		}

		return
	}

	if err := h.auditLogService.Create(
		currentUser.ID.String(),
		"CREATE",
		"users",
		user.ID.String(),
		"User created successfully",
		c.ClientIP(),
	); err != nil {
		// Audit logging failure must not fail the user creation.
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "User created successfully",
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
			"role":     user.Role.Name,
			"status":   user.Status,
		},
	})
}

// List returns users with search, role filter and pagination.
//
// @Summary List users
// @Description Retrieve users with search, role filtering and pagination.
// @Tags Users
// @Produce json
// @Param search query string false "Search users"
// @Param role query string false "Filter by role"
// @Param page query int false "Page number"
// @Param page_size query int false "Number of users per page"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /users [get]
func (h *UserHandler) List(c *gin.Context) {
	var query models.UserListQuery

	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid query parameters",
		})
		return
	}

	users, total, page, pageSize, err :=
		h.userService.ListUsers(query)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Unable to retrieve users",
		})
		return
	}

	userItems := make([]gin.H, 0, len(users))

	for _, user := range users {
		userItems = append(userItems, gin.H{
			"id":            user.ID,
			"username":      user.Username,
			"email":         user.Email,
			"role":          user.Role.Name,
			"status":        user.Status,
			"last_login_at": user.LastLoginAt,
			"created_at":    user.CreatedAt,
			"updated_at":    user.UpdatedAt,
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
		"message": "Users retrieved successfully",
		"data":    userItems,
		"pagination": gin.H{
			"page":        page,
			"page_size":   pageSize,
			"total_items": total,
			"total_pages": totalPages,
		},
	})
}

// GetByID returns one user by UUID.
//
// @Summary Get user
// @Description Retrieve a single user by UUID.
// @Tags Users
// @Produce json
// @Param id path string true "User UUID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /users/{id} [get]
func (h *UserHandler) GetByID(c *gin.Context) {
	userID := c.Param("id")

	user, err := h.userService.GetUserByID(userID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidUserID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": constants.ErrInvalidUserID,
			})

		case errors.Is(err, repository.ErrUserNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": constants.ErrUserNotFound,
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to retrieve user",
			})
		}

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User retrieved successfully",
		"user": gin.H{
			"id":            user.ID,
			"username":      user.Username,
			"email":         user.Email,
			"role":          user.Role.Name,
			"status":        user.Status,
			"last_login_at": user.LastLoginAt,
			"created_at":    user.CreatedAt,
			"updated_at":    user.UpdatedAt,
		},
	})
}

// Update updates an existing system user.
//
// @Summary Update user
// @Description Update an existing system user's information.
// @Tags Users
// @Accept json
// @Produce json
// @Param id path string true "User UUID"
// @Param request body models.UpdateUserRequest true "Updated user information"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{}
// @Failure 422 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /users/{id} [put]
func (h *UserHandler) Update(c *gin.Context) {
	userID := c.Param("id")

	var request models.UpdateUserRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid user information",
			"error":   err.Error(),
		})
		return
	}

	currentUser, ok := h.getCurrentUser(c)
	if !ok {
		return
	}

	if currentUser.Role.Name != models.RoleSuperAdmin &&
		request.Role == models.RoleSuperAdmin {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": constants.ErrInvalidRoleAssignment,
		})
		return
	}

	user, err := h.userService.UpdateUser(userID, request)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidUserID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": constants.ErrInvalidUserID,
			})

		case errors.Is(err, repository.ErrUserNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": constants.ErrUserNotFound,
			})

		case errors.Is(err, services.ErrUserAlreadyExists):
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"message": constants.ErrUserAlreadyExists,
			})

		case errors.Is(err, services.ErrInvalidRole):
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"success": false,
				"message": constants.ErrInvalidRole,
			})

		case errors.Is(err, services.ErrInvalidUserStatus):
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"success": false,
				"message": constants.ErrInvalidUserStatus,
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to update user",
			})
		}

		return
	}

	if err := h.auditLogService.Create(
		currentUser.ID.String(),
		"UPDATE",
		"users",
		user.ID.String(),
		"User updated successfully",
		c.ClientIP(),
	); err != nil {
		// Audit logging failure must not fail the user update.
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User updated successfully",
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
			"role":     user.Role.Name,
			"status":   user.Status,
		},
	})
}

// UpdateStatus updates the status of a user.
//
// @Summary Update user status
// @Description Change the status of an existing user.
// @Tags Users
// @Accept json
// @Produce json
// @Param id path string true "User UUID"
// @Param request body models.UpdateUserStatusRequest true "User status"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 422 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /users/{id}/status [patch]
func (h *UserHandler) UpdateStatus(c *gin.Context) {
	userID := c.Param("id")

	var request models.UpdateUserStatusRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Status is required",
		})
		return
	}

	currentUser, ok := h.getCurrentUser(c)
	if !ok {
		return
	}

	err := h.userService.UpdateUserStatus(
		userID,
		request.Status,
	)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidUserID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": constants.ErrInvalidUserID,
			})

		case errors.Is(err, services.ErrInvalidUserStatus):
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"success": false,
				"message": constants.ErrInvalidUserStatus,
			})

		case errors.Is(err, repository.ErrUserNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": constants.ErrUserNotFound,
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to update user status",
			})
		}

		return
	}

	if err := h.auditLogService.Create(
		currentUser.ID.String(),
		"STATUS_CHANGE",
		"users",
		userID,
		"User status changed to "+request.Status,
		c.ClientIP(),
	); err != nil {
		// Audit logging failure must not fail the status update.
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User status updated successfully",
	})
}

// ResetPassword resets a user's password.
//
// @Summary Reset user password
// @Description Reset the password of an existing user.
// @Tags Users
// @Accept json
// @Produce json
// @Param id path string true "User UUID"
// @Param request body models.ResetUserPasswordRequest true "New password"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 422 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /users/{id}/password [patch]
func (h *UserHandler) ResetPassword(c *gin.Context) {
	userID := c.Param("id")

	var request models.ResetUserPasswordRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "A valid new password is required",
		})
		return
	}

	err := h.userService.ResetPassword(
		userID,
		request.NewPassword,
	)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidUserID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": constants.ErrInvalidUserID,
			})

		case errors.Is(err, services.ErrInvalidPassword):
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"success": false,
				"message": constants.ErrInvalidPassword,
			})

		case errors.Is(err, repository.ErrUserNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": constants.ErrUserNotFound,
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to reset user password",
			})
		}

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User password reset successfully",
	})
}

// Delete deletes a user.
//
// @Summary Delete user
// @Description Delete an existing user.
// @Tags Users
// @Produce json
// @Param id path string true "User UUID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /users/{id} [delete]
func (h *UserHandler) Delete(c *gin.Context) {
	targetUserID := c.Param("id")

	currentUser, ok := h.getCurrentUser(c)
	if !ok {
		return
	}

	err := h.userService.DeleteUser(
		targetUserID,
		currentUser.ID.String(),
	)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidUserID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": constants.ErrInvalidUserID,
			})

		case errors.Is(err, services.ErrCannotDeleteSelf):
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": constants.ErrCannotDeleteSelf,
			})

		case errors.Is(err, repository.ErrUserNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": constants.ErrUserNotFound,
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to delete user",
			})
		}

		return
	}

	if err := h.auditLogService.Create(
		currentUser.ID.String(),
		"DELETE",
		"users",
		targetUserID,
		"User deleted successfully",
		c.ClientIP(),
	); err != nil {
		// Audit logging failure must not fail the user deletion.
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User deleted successfully",
	})
}

// getCurrentUser returns the authenticated user from the Gin context.
func (h *UserHandler) getCurrentUser(
	c *gin.Context,
) (*models.User, bool) {
	currentUserValue, exists := c.Get("current_user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": constants.ErrAuthenticationRequired,
		})
		return nil, false
	}

	currentUser, ok := currentUserValue.(*models.User)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": constants.ErrInvalidAuthContext,
		})
		return nil, false
	}

	return currentUser, true
}
