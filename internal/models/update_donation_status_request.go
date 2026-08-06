package models

type UpdateDonationStatusRequest struct {
	Status string `json:"status" binding:"required"`
}
