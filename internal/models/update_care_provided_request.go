package models

type UpdateCareProvidedRequest struct {
	Amount      float64 `json:"amount" binding:"required,gt=0"`
	Description string  `json:"description" binding:"omitempty,max=1000"`
	ProvidedAt  string  `json:"provided_at" binding:"required"`
}
