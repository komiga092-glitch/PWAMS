package models

type UpdateCareProvidedRequest struct {
	Amount      float64 `json:"amount" binding:"required,gt=0"`
	Description string  `json:"description" binding:"omitempty,max=1000"`
	CareType    string  `json:"care_type" binding:"required"`
	ProvidedBy  string  `json:"provided_by" binding:"omitempty,max=150"`
	ProvidedAt  string  `json:"provided_at" binding:"required"`
}
