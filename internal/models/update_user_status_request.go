package models

type UpdateUserStatusRequest struct {
	Status string `json:"status" binding:"required"`
}
