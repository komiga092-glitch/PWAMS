package models

type UpdateDonorStatusRequest struct {
	Status string `json:"status" binding:"required"`
}
