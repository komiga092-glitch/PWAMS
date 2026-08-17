package models

type UpdateCareProvidedStatusRequest struct {
	Status string `json:"status" binding:"required"`
}
