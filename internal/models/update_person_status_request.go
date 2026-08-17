package models

type UpdatePersonStatusRequest struct {
	Status string `json:"status" binding:"required"`
}
