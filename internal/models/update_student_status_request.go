package models

type UpdateStudentStatusRequest struct {
	Status string `json:"status" binding:"required"`
}
