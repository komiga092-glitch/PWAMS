package models

type CreateCareProvidedRequest struct {
	AidRequestID string  `json:"aid_request_id" binding:"required,uuid"`
	PersonID     string  `json:"person_id" binding:"required,uuid"`
	Amount       float64 `json:"amount" binding:"required,gt=0"`
	Description  string  `json:"description" binding:"omitempty,max=1000"`
	ProvidedAt   string  `json:"provided_at" binding:"required"`
}
