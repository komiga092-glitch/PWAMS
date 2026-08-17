package handlers

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/repository"
	"github.com/komiga092-glitch/pwams/internal/services"
)

const (
	uploadDirectory = "storage/uploads"
	maxUploadSize   = 2 * 1024 * 1024
)

type FileUploadHandler struct {
	fileUploadService *services.FileUploadService
}

func NewFileUploadHandler(
	fileUploadService *services.FileUploadService,
) *FileUploadHandler {
	return &FileUploadHandler{
		fileUploadService: fileUploadService,
	}
}

// Upload handles file uploads.
func (h *FileUploadHandler) Upload(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "File is required",
		})
		return
	}

	if fileHeader.Size <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "File is empty",
		})
		return
	}

	if fileHeader.Size > maxUploadSize {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"success": false,
			"message": "File size must not exceed 2 MB",
		})
		return
	}

	allowedTypes := map[string]bool{
		"image/jpeg":      true,
		"image/png":       true,
		"image/webp":      true,
		"application/pdf": true,
	}

	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Unable to read uploaded file",
		})
		return
	}
	defer file.Close()

	buffer := make([]byte, 512)

	bytesRead, err := file.Read(buffer)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Unable to inspect uploaded file",
		})
		return
	}

	contentType := http.DetectContentType(buffer[:bytesRead])

	if !allowedTypes[contentType] {
		c.JSON(http.StatusUnsupportedMediaType, gin.H{
			"success":      false,
			"message":      "Unsupported file type",
			"content_type": contentType,
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

	if err := os.MkdirAll(uploadDirectory, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Unable to prepare upload directory",
		})
		return
	}

	extension := strings.ToLower(filepath.Ext(fileHeader.Filename))
	storedName := uuid.NewString() + extension
	storedPath := filepath.Join(uploadDirectory, storedName)

	if err := c.SaveUploadedFile(fileHeader, storedPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Unable to save uploaded file",
		})
		return
	}

	fileUpload := &models.FileUpload{
		ID:           uuid.New(),
		UserID:       currentUser.ID,
		OriginalName: filepath.Base(fileHeader.Filename),
		StoredName:   storedName,
		Path:         storedPath,
		ContentType:  contentType,
		Size:         fileHeader.Size,
	}

	if err := h.fileUploadService.Create(fileUpload); err != nil {
		_ = os.Remove(storedPath)

		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Unable to save file information",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "File uploaded successfully",
		"file": gin.H{
			"id":            fileUpload.ID,
			"original_name": fileUpload.OriginalName,
			"content_type":  fileUpload.ContentType,
			"size":          fileUpload.Size,
			"created_at":    fileUpload.CreatedAt,
		},
	})
}

// Download returns a file only to its owner.
func (h *FileUploadHandler) Download(c *gin.Context) {
	fileID := c.Param("id")

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

	fileUpload, err := h.fileUploadService.GetByIDForUser(
		fileID,
		currentUser.ID.String(),
	)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidFileUpload):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid file ID",
			})

		case errors.Is(err, repository.ErrFileUploadNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "File not found",
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to retrieve file",
			})
		}

		return
	}

	c.FileAttachment(
		fileUpload.Path,
		fileUpload.OriginalName,
	)
}

// Delete deletes a file only if it belongs to the current user.
func (h *FileUploadHandler) Delete(c *gin.Context) {
	fileID := c.Param("id")

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

	if err := h.fileUploadService.DeleteForUser(
		fileID,
		currentUser.ID.String(),
	); err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidFileUpload):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid file ID",
			})

		case errors.Is(err, repository.ErrFileUploadNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "File not found",
			})

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to delete file",
			})
		}

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "File deleted successfully",
	})
}
