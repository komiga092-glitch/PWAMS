package handlers

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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
	auditLogService   *services.AuditLogService
}

func NewFileUploadHandler(
	fileUploadService *services.FileUploadService,
	auditLogServices ...*services.AuditLogService,
) *FileUploadHandler {
	var auditLogService *services.AuditLogService
	if len(auditLogServices) > 0 {
		auditLogService = auditLogServices[0]
	}
	return &FileUploadHandler{
		fileUploadService: fileUploadService,
		auditLogService:   auditLogService,
	}
}

func (h *FileUploadHandler) Page(c *gin.Context) {
	c.HTML(http.StatusOK, "base", gin.H{
		"page_template": "files_content",
		"title":         "File Management",
	})
}

func (h *FileUploadHandler) List(c *gin.Context) {
	currentUser, ok := getCurrentUser(c)
	if !ok {
		return
	}

	page, pageSize, valid := filePagination(c)
	if !valid {
		return
	}
	files, total, page, pageSize, err := h.fileUploadService.ListForUser(
		currentUser.ID.String(), page, pageSize,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Unable to retrieve files"})
		return
	}

	items := make([]gin.H, 0, len(files))
	for _, file := range files {
		items = append(items, gin.H{
			"id":            file.ID,
			"original_name": file.OriginalName,
			"content_type":  file.ContentType,
			"size":          file.Size,
			"created_at":    file.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"data":       items,
		"pagination": buildPagination(total, page, pageSize),
	})
}

func (h *FileUploadHandler) Reconcile(c *gin.Context) {
	result, err := h.fileUploadService.Reconcile(uploadDirectory)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Unable to reconcile file storage",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

// Upload handles file uploads.
func (h *FileUploadHandler) Upload(c *gin.Context) {
	idempotencyKey := strings.TrimSpace(c.GetHeader("Idempotency-Key"))

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

	if idempotencyKey != "" {
		existing, err := h.fileUploadService.FindByUserAndIdempotencyKey(
			currentUser.ID,
			idempotencyKey,
		)
		if err == nil {
			h.writeUploadSuccess(c, existing, http.StatusOK)
			return
		}
		if !errors.Is(err, repository.ErrFileUploadNotFound) {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to check upload idempotency",
			})
			return
		}
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"success": false,
				"message": "Upload request must not exceed 2 MB plus multipart overhead",
			})
			return
		}

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

	content, err := io.ReadAll(io.LimitReader(file, maxUploadSize+1))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Unable to inspect uploaded file",
		})
		return
	}
	if int64(len(content)) > maxUploadSize {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"success": false,
			"message": "File size must not exceed 2 MB",
		})
		return
	}

	contentType := http.DetectContentType(content[:minInt(len(content), 512)])

	if !allowedTypes[contentType] {
		c.JSON(http.StatusUnsupportedMediaType, gin.H{
			"success":      false,
			"message":      "Unsupported file type",
			"content_type": contentType,
		})
		return
	}

	if err := validateUploadContent(contentType, content); err != nil {
		c.JSON(http.StatusUnsupportedMediaType, gin.H{
			"success": false,
			"message": err.Error(),
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

	var persistedIdempotencyKey *string
	if idempotencyKey != "" {
		persistedIdempotencyKey = &idempotencyKey
	}

	fileUpload := &models.FileUpload{
		ID:             uuid.New(),
		UserID:         currentUser.ID,
		IdempotencyKey: persistedIdempotencyKey,
		OriginalName:   filepath.Base(fileHeader.Filename),
		StoredName:     storedName,
		Path:           storedPath,
		ContentType:    contentType,
		Size:           fileHeader.Size,
		SHA256:         sha256Hex(content),
	}

	if err := h.fileUploadService.Create(fileUpload); err != nil {
		_ = os.Remove(storedPath)

		if idempotencyKey != "" {
			existing, lookupErr := h.fileUploadService.FindByUserAndIdempotencyKey(
				currentUser.ID,
				idempotencyKey,
			)
			if lookupErr == nil {
				h.writeUploadSuccess(c, existing, http.StatusOK)
				return
			}
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Unable to save file information",
		})
		return
	}
	h.audit("UPLOAD", fileUpload, currentUser.ID.String(), c)

	h.writeUploadSuccess(c, fileUpload, http.StatusCreated)
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func sha256Hex(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}

func validateUploadContent(contentType string, content []byte) error {
	switch contentType {
	case "image/jpeg", "image/png":
		if _, _, err := image.Decode(bytes.NewReader(content)); err != nil {
			return fmt.Errorf("uploaded image could not be decoded")
		}
	case "image/webp":
		if len(content) < 12 ||
			!bytes.Equal(content[:4], []byte("RIFF")) ||
			!bytes.Equal(content[8:12], []byte("WEBP")) {
			return fmt.Errorf("uploaded WebP image has an invalid structure")
		}
	case "application/pdf":
		if len(content) < 5 || !bytes.Equal(content[:5], []byte("%PDF-")) {
			return fmt.Errorf("uploaded PDF has an invalid structure")
		}
	}

	return nil
}

func (h *FileUploadHandler) writeUploadSuccess(
	c *gin.Context,
	fileUpload *models.FileUpload,
	status int,
) {
	c.JSON(status, gin.H{
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

		case errors.Is(err, repository.ErrFileUploadNotFound),
			errors.Is(err, services.ErrFileUploadAlreadyDeleted):
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
	if _, statErr := os.Stat(fileUpload.Path); statErr != nil {
		if errors.Is(statErr, os.ErrNotExist) {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "File content not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Unable to access file"})
		return
	}
	h.audit("DOWNLOAD", fileUpload, currentUser.ID.String(), c)

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

		case errors.Is(err, repository.ErrFileUploadNotFound),
			errors.Is(err, services.ErrFileUploadAlreadyDeleted):
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
	h.audit("DELETE", nil, currentUser.ID.String(), c, fileID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "File deleted successfully",
	})
}

func (h *FileUploadHandler) audit(action string, file *models.FileUpload, userID string, c *gin.Context, ids ...string) {
	if h.auditLogService == nil {
		return
	}
	entityID := ""
	details := "File " + action + " completed"
	if file != nil {
		entityID = file.ID.String()
		details = "File " + action + ": " + file.OriginalName
	} else if len(ids) > 0 {
		entityID = ids[0]
	}
	_ = h.auditLogService.Create(userID, action, "file_uploads", entityID, details, c.ClientIP())
}

func filePagination(c *gin.Context) (int, int, bool) {
	page := 1
	pageSize := 20
	if value := c.Query("page"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid pagination parameters"})
			return 0, 0, false
		}
		page = parsed
	}
	if value := c.Query("page_size"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid pagination parameters"})
			return 0, 0, false
		}
		pageSize = parsed
	}
	return page, pageSize, true
}
