package models

type CancelAidRequest struct {
	Reason string `json:"reason" binding:"required,min=3"`
}
