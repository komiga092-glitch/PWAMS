package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

type UserHandler struct {
	userService *services.UserService
}

func NewUserHandler(userService *services.UserService) *UserHandler {
	return &UserHandler{
		userService: userService,
	}
}

// Create creates a new system user.
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
func (h *UserHandler) GetByID(c *gin.Context) {
	userID := c.Param("id")

	user, err := h.userService.GetUserByID(userID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidUserID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid user ID",
			})

		case errors.Is(err, repository.ErrUserNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "User not found",
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

	if currentUser.Role.Name != models.RoleSuperAdmin &&
		request.Role == models.RoleSuperAdmin {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "Only Super Admin can assign the Super Admin role",
		})
		return
	}

	user, err := h.userService.UpdateUser(userID, request)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidUserID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid user ID",
			})

		case errors.Is(err, repository.ErrUserNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "User not found",
			})

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

		case errors.Is(err, services.ErrInvalidUserStatus):
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"success": false,
				"message": err.Error(),
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to update user",
			})
		}

		return
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

	err := h.userService.UpdateUserStatus(
		userID,
		request.Status,
	)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidUserID):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid user ID",
			})

		case errors.Is(err, services.ErrInvalidUserStatus):
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"success": false,
				"message": err.Error(),
			})

		case errors.Is(err, repository.ErrUserNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "User not found",
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to update user status",
			})
		}

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User status updated successfully",
	})
}

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
				"message": "Invalid user ID",
			})

		case errors.Is(err, services.ErrInvalidPassword):
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"success": false,
				"message": err.Error(),
			})

		case errors.Is(err, repository.ErrUserNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "User not found",
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
